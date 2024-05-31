package proto

// Adds Foobar() method to NestedMessage
func (m *NestedMessage) Foobar() {}

// Provides details of NestedMessage
type NestedMessageDetails struct {
	ID          string
	Name        string
	Status      string
	Description string
}

func (m *NestedMessage) ToDetails() NestedMessageDetails {
	details := NestedMessageDetails{
		ID:     m.Id,
		Name:   m.Name,
		Status: m.Status.String(),
	}
	if m.Details != nil {
		details.Description = m.GetDescription()
	}
	return details
}
