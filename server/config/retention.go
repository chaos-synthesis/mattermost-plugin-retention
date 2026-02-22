package config

import (
	"fmt"
	"strconv"
	"time"
)

const (
	FullLayout      = "Jan 2, 2006 3:04pm -0700"
	TimeOfDayLayout = "3:04pm -0700"
)

type RetentionJobSettings struct {
	EnableRetentionPolicy bool
	Frequency             Frequency
	DayOfWeek             int
	TimeOfDay             time.Time
	BatchSize             int
}

func (c *RetentionJobSettings) Clone() *RetentionJobSettings {
	return &RetentionJobSettings{
		EnableRetentionPolicy: c.EnableRetentionPolicy,
		Frequency:             c.Frequency,
		TimeOfDay:             c.TimeOfDay,
		BatchSize:             c.BatchSize,
	}
}

func (c *RetentionJobSettings) String() string {
	return fmt.Sprintf("enabled=%T; freq=%s; tod=%s; batchSize=%d",
		c.EnableRetentionPolicy, c.Frequency, c.TimeOfDay.Format(TimeOfDayLayout), c.BatchSize)
}

func (c *Configuration) GetPostRetentionJobSettings() (*RetentionJobSettings, error) {
	if !c.EnableRetentionPolicy {
		return &RetentionJobSettings{
			EnableRetentionPolicy: false,
		}, nil
	}

	freq, err := FreqFromString(c.Frequency)
	if err != nil {
		return nil, err
	}

	dow, err := ParseInt(c.DayOfWeek, 0, 6)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `Day of week`: %w", err)
	}

	tod, err := time.Parse(TimeOfDayLayout, c.TimeOfDay)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `Time of day`: %w", err)
	}

	batchSize := c.BatchSize
	if batchSize < MinBatchSize {
		batchSize = MinBatchSize
	}
	if batchSize > MaxBatchSize {
		batchSize = MaxBatchSize
	}

	return &RetentionJobSettings{
		EnableRetentionPolicy: c.EnableRetentionPolicy,
		Frequency:             freq,
		DayOfWeek:             dow,
		TimeOfDay:             tod,
		BatchSize:             batchSize,
	}, nil
}

func ParseInt(s string, minVal int, maxVal int) (int, error) {
	i64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	i := int(i64)

	if i < minVal {
		return 0, fmt.Errorf("number must be greater than or equal to %d", minVal)
	}

	if i > maxVal {
		return 0, fmt.Errorf("number must be less than or equal to %d", maxVal)
	}
	return i, nil
}
