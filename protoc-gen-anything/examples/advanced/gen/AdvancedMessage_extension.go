package proto

// Adds Foobar() method to AdvancedMessage
func (m *AdvancedMessage) Foobar() {}

// Provides details of AdvancedMessage
type AdvancedMessageDetails struct {
	ID          string
	Name        string
	Status      string
	Description string
}

func (m *AdvancedMessage) ToDetails() AdvancedMessageDetails {
	details := AdvancedMessageDetails{
		ID:     m.Id,
		Name:   m.Name,
		Status: m.Status.String(),
	}
	if m.Details != nil {
		details.Description = m.GetDescription()
	}
	return details
}
