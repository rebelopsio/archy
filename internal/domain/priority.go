package domain

// Priority is the canonical priority level used across all providers.
// Provider-specific priority schemes (Linear's 0-4, GitHub labels, etc.)
// are mapped into this enum at the adapter boundary.
type Priority int

// Priority values, low to high.
const (
	// PriorityUnknown means no priority is set or the provider's value
	// could not be mapped. The default zero value.
	PriorityUnknown Priority = iota
	// PriorityLow is the lowest non-unknown priority.
	PriorityLow
	// PriorityMedium is the default working priority for most items.
	PriorityMedium
	// PriorityHigh marks items that should be addressed soon.
	PriorityHigh
	// PriorityUrgent marks items that need immediate attention.
	PriorityUrgent
)

// String returns the priority's lowercase name. Out-of-range values
// return "unknown".
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return "urgent"
	case PriorityUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}
