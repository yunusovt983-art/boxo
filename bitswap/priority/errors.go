package priority

import "errors"

var (
	// ErrQueueFull is returned when a priority queue is at capacity
	ErrQueueFull = errors.New("priority queue is full")
	
	// ErrInvalidPriority is returned when an invalid priority level is specified
	ErrInvalidPriority = errors.New("invalid priority level")
	
	// ErrResourcesUnavailable is returned when insufficient resources are available
	ErrResourcesUnavailable = errors.New("insufficient resources available")
	
	// ErrManagerStopped is returned when operations are attempted on a stopped manager
	ErrManagerStopped = errors.New("priority manager has been stopped")
	
	// ErrRequestTimeout is returned when a request times out
	ErrRequestTimeout = errors.New("request processing timeout")
	
	// ErrInvalidConfiguration is returned when invalid configuration is provided
	ErrInvalidConfiguration = errors.New("invalid configuration parameters")
)