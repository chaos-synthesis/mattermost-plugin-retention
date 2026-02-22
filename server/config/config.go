package config

const (
	DefaultBatchSize = 50
	//DefaultListBatchSize    = 1000

	MinBatchSize = 10
	MaxBatchSize = 1000
)

// Configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type Configuration struct {
	EnableRetentionPolicy bool
	// Frequency is the frequency at which the plugin will run the retention policy.
	Frequency string
	// DayOfWeek is the day of the week on which the plugin will run the retention policy.
	DayOfWeek string
	// TimeOfDay is the time of day at which the plugin will run the retention policy.
	TimeOfDay string
	// BatchSize is the number of posts to delete in each batch when running the retention policy.
	BatchSize int
}

func NewConfiguration() *Configuration {
	return &Configuration{
		BatchSize: DefaultBatchSize,
	}
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *Configuration) Clone() *Configuration {
	clone := *c
	return &clone
}
