package viz

import (
	"context"
	"fmt"
	"sync"

	commonpb "go.viam.com/api/common/v1"
	wsspb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// DefaultChangeBufferSize is the default capacity of the
// TransformChange channel. 128 covers the worst-case "operator hits
// reset on a 100-box pallet" burst without blocking emit().
const DefaultChangeBufferSize = 128

// StoreOption configures a Store at construction.
type StoreOption func(*Store)

// WithChangeBufferSize overrides the change-channel capacity.
// Defaults to DefaultChangeBufferSize. Set higher for producers that
// emit large bursts (e.g. clearing 1000 placed boxes); set lower if
// memory matters more than burst tolerance.
func WithChangeBufferSize(n int) StoreOption {
	return func(s *Store) { s.bufferSize = n }
}

// OnDropped sets an optional callback invoked when the change channel
// is full and an emit is dropped. Use it to log a warning. Defaults
// to a silent drop.
//
// Subscribers can resync after a drop via ListUUIDs / GetTransform —
// the in-memory state stays authoritative.
func OnDropped(cb func(uuid string)) StoreOption {
	return func(s *Store) { s.onDropped = cb }
}

// Store is an in-memory WorldStateStore-style Transform registry.
// Producers call Set / Remove / Clear to mutate the live set; the
// WorldStateStore RPC methods (ListUUIDs, GetTransform,
// StreamTransformChanges) read from the same backing map.
//
// Set with a fresh UUID emits ADDED; Set with an existing UUID emits
// REPLACED. Remove with an existing UUID emits REMOVED; Remove of an
// unknown UUID is a no-op (no event). Clear emits one REMOVED per
// existing UUID, then drops them all.
//
// Concurrency: all methods are safe to call from any goroutine. The
// internal mutex is released before any event is enqueued, so a slow
// subscriber can't block Store mutations beyond the channel buffer.
type Store struct {
	mu         sync.Mutex
	transforms map[string]*commonpb.Transform
	changeChan chan worldstatestore.TransformChange
	bufferSize int
	onDropped  func(uuid string)
}

// NewStore constructs a Store with the given options.
func NewStore(opts ...StoreOption) *Store {
	s := &Store{
		transforms: map[string]*commonpb.Transform{},
		bufferSize: DefaultChangeBufferSize,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.changeChan = make(chan worldstatestore.TransformChange, s.bufferSize)
	return s
}

// Set inserts or replaces a Transform. ADDED is emitted when the UUID
// is new; REPLACED when it already existed.
//
// The transform's UUID is the identity key. Passing a Transform with
// an empty UUID is a no-op (no event, no insert) — UUIDs are required.
func (s *Store) Set(tr *commonpb.Transform) {
	if tr == nil || len(tr.Uuid) == 0 {
		return
	}
	uuid := string(tr.Uuid)
	s.mu.Lock()
	_, existed := s.transforms[uuid]
	s.transforms[uuid] = tr
	s.mu.Unlock()

	changeType := wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED
	if existed {
		changeType = wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED
	}
	s.emit(worldstatestore.TransformChange{
		ChangeType: changeType,
		Transform:  tr,
	})
}

// Remove drops the Transform for `uuid` and emits REMOVED. No-op
// (no event) when the UUID is unknown.
func (s *Store) Remove(uuid string) {
	s.mu.Lock()
	_, existed := s.transforms[uuid]
	if existed {
		delete(s.transforms, uuid)
	}
	s.mu.Unlock()
	if !existed {
		return
	}
	s.emit(worldstatestore.TransformChange{
		ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
		Transform:  &commonpb.Transform{Uuid: []byte(uuid)},
	})
}

// Clear emits REMOVED for every entry and resets the store to empty.
func (s *Store) Clear() {
	s.mu.Lock()
	uuids := make([]string, 0, len(s.transforms))
	for u := range s.transforms {
		uuids = append(uuids, u)
	}
	s.transforms = map[string]*commonpb.Transform{}
	s.mu.Unlock()
	for _, u := range uuids {
		s.emit(worldstatestore.TransformChange{
			ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
			Transform:  &commonpb.Transform{Uuid: []byte(u)},
		})
	}
}

// Len returns the number of transforms currently in the store.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.transforms)
}

// ListUUIDs implements worldstatestore.Service.ListUUIDs.
func (s *Store) ListUUIDs(_ context.Context, _ map[string]any) ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, 0, len(s.transforms))
	for u := range s.transforms {
		out = append(out, []byte(u))
	}
	return out, nil
}

// GetTransform implements worldstatestore.Service.GetTransform.
func (s *Store) GetTransform(_ context.Context, uuid []byte, _ map[string]any) (*commonpb.Transform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tr, ok := s.transforms[string(uuid)]
	if !ok {
		return nil, fmt.Errorf("viz.Store: unknown uuid %q", string(uuid))
	}
	return tr, nil
}

// StreamTransformChanges implements
// worldstatestore.Service.StreamTransformChanges.
func (s *Store) StreamTransformChanges(ctx context.Context, _ map[string]any) (*worldstatestore.TransformChangeStream, error) {
	return worldstatestore.NewTransformChangeStreamFromChannel(ctx, s.changeChan), nil
}

// emit puts a change on the buffered channel; drops + invokes
// onDropped (if set) when the buffer is full so a slow subscriber
// can't block mutations.
func (s *Store) emit(c worldstatestore.TransformChange) {
	select {
	case s.changeChan <- c:
	default:
		if s.onDropped != nil {
			s.onDropped(string(c.Transform.GetUuid()))
		}
	}
}

// ---------------------------------------------------------------------------
// Ready-to-register service
// ---------------------------------------------------------------------------

// StoreService is a worldstatestore.Service backed by a Store. Drop
// it into a module's service constructor when you want a generic
// "scribble transforms into me from outside" service with no custom
// logic. Custom DoCommand verbs / config attributes go in a thin
// wrapper around it.
//
// Example minimal module:
//
//	type cfg struct { ObserverFrame string `json:"observer_frame"` }
//	func (c *cfg) Validate(string) ([]string, []string, error) { return nil, nil, nil }
//
//	resource.RegisterService(worldstatestore.API,
//	    resource.NewModel("acme", "scratch", "wss"),
//	    resource.Registration[worldstatestore.Service, *cfg]{
//	        Constructor: func(_ context.Context, _ resource.Dependencies,
//	            conf resource.Config, _ logging.Logger) (worldstatestore.Service, error) {
//	            return viz.NewStoreService(conf.ResourceName()), nil
//	        },
//	    })
type StoreService struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name resource.Name
	*Store
}

// NewStoreService constructs a StoreService named `name`, sharing
// store options with NewStore.
func NewStoreService(name resource.Name, opts ...StoreOption) *StoreService {
	return &StoreService{
		name:  name,
		Store: NewStore(opts...),
	}
}

// Name returns the service's resource name (for the resource.Resource
// interface that worldstatestore.Service embeds).
func (s *StoreService) Name() resource.Name { return s.name }

// DoCommand exposes set / remove / clear verbs so callers can mutate
// the store without having to instantiate a custom DoCommand surface.
// Verbs:
//
//	{"set": {<transform-as-map>}}      → ADDED/UPDATED
//	{"remove": "<uuid>"}               → REMOVED
//	{"remove": ["<uuid>", ...]}        → REMOVED for each
//	{"clear": true}                    → REMOVED for every entry
//	{"len": true}                      → {"count": N}
//
// The "set" verb's transform shape follows the commonpb.Transform
// JSON encoding (uuid, reference_frame, pose_in_observer_frame, etc.).
// For callers building transforms from scratch, easier to construct
// a viz.Box / viz.Sphere via Go and call s.Set(tr.ToTransform())
// directly than to marshal through DoCommand.
func (s *StoreService) DoCommand(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := cmd["clear"]; ok {
		s.Clear()
		return map[string]interface{}{"cleared": true}, nil
	}
	if v, ok := cmd["remove"]; ok {
		switch u := v.(type) {
		case string:
			s.Remove(u)
			return map[string]interface{}{"removed": 1}, nil
		case []interface{}:
			n := 0
			for _, item := range u {
				if str, ok := item.(string); ok {
					s.Remove(str)
					n++
				}
			}
			return map[string]interface{}{"removed": n}, nil
		default:
			return nil, fmt.Errorf("remove: expected string or string array, got %T", v)
		}
	}
	if _, ok := cmd["len"]; ok {
		return map[string]interface{}{"count": s.Len()}, nil
	}
	return nil, fmt.Errorf("viz.StoreService: unknown command %v", cmd)
}
