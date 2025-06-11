package digraph

import (
	"fmt"
	"strconv"
	"strings"
)

// Resources represents resource requirements for a DAG or step (internal representation)
type Resources struct {
	MemoryRequestBytes int64 `json:"memoryRequestBytes,omitempty"`
	MemoryLimitBytes   int64 `json:"memoryLimitBytes,omitempty"`
	CPURequestMillis   int   `json:"cpuRequestMillis,omitempty"`
	CPULimitMillis     int   `json:"cpuLimitMillis,omitempty"`
}

// ResourcesConfig represents the YAML configuration for resources
type ResourcesConfig struct {
	Requests *ResourceQuantities `yaml:"requests,omitempty" json:"requests,omitempty"`
	Limits   *ResourceQuantities `yaml:"limits,omitempty" json:"limits,omitempty"`
}

// ResourceQuantities represents CPU and memory quantities in YAML
type ResourceQuantities struct {
	CPU    string `yaml:"cpu,omitempty" json:"cpu,omitempty"`       // CPU cores (e.g., "0.5", "2")
	Memory string `yaml:"memory,omitempty" json:"memory,omitempty"` // Memory (e.g., "1Gi", "512Mi")
}

// ResourceUnit represents different units for resources
type ResourceUnit int

const (
	UnitByte ResourceUnit = iota
	UnitKilobyte
	UnitMegabyte
	UnitGigabyte
	UnitTerabyte
	UnitKibibyte
	UnitMebibyte
	UnitGibibyte
	UnitTebibyte
)

// Unit multipliers
const (
	Byte = 1

	// Decimal units (SI)
	Kilobyte = 1000 * Byte
	Megabyte = 1000 * Kilobyte
	Gigabyte = 1000 * Megabyte
	Terabyte = 1000 * Gigabyte

	// Binary units (IEC)
	Kibibyte = 1024 * Byte
	Mebibyte = 1024 * Kibibyte
	Gibibyte = 1024 * Mebibyte
	Tebibyte = 1024 * Gibibyte
)

// ParseResourcesConfig converts ResourcesConfig to internal Resources representation
func ParseResourcesConfig(config *ResourcesConfig) (*Resources, error) {
	if config == nil {
		return &Resources{}, nil
	}

	resources := &Resources{}

	// Parse requests
	if config.Requests != nil {
		cpuMillis, err := ParseCPUToMillis(config.Requests.CPU)
		if err != nil {
			return nil, fmt.Errorf("parsing request CPU: %w", err)
		}
		resources.CPURequestMillis = cpuMillis

		memBytes, err := ParseMemory(config.Requests.Memory)
		if err != nil {
			return nil, fmt.Errorf("parsing request memory: %w", err)
		}
		resources.MemoryRequestBytes = memBytes
	}

	// Parse limits
	if config.Limits != nil {
		cpuMillis, err := ParseCPUToMillis(config.Limits.CPU)
		if err != nil {
			return nil, fmt.Errorf("parsing limit CPU: %w", err)
		}
		resources.CPULimitMillis = cpuMillis

		memBytes, err := ParseMemory(config.Limits.Memory)
		if err != nil {
			return nil, fmt.Errorf("parsing limit memory: %w", err)
		}
		resources.MemoryLimitBytes = memBytes
	}

	return resources, nil
}

// ParseCPUToMillis parses a CPU quantity string into millicores
func ParseCPUToMillis(cpu string) (int, error) {
	if cpu == "" {
		return 0, nil
	}

	// Remove any whitespace
	cpu = strings.TrimSpace(cpu)

	// Handle millicores (e.g., "500m" = 500 millicores)
	if strings.HasSuffix(cpu, "m") {
		milliStr := strings.TrimSuffix(cpu, "m")
		milli, err := strconv.Atoi(milliStr)
		if err != nil {
			return 0, fmt.Errorf("invalid CPU quantity: %s", cpu)
		}
		return milli, nil
	}

	// Parse as regular float and convert to millicores
	cores, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU quantity: %s", cpu)
	}

	return int(cores * 1000), nil
}

// ParseCPU parses a CPU quantity string into CPU cores
func ParseCPU(cpu string) (float64, error) {
	if cpu == "" {
		return 0, nil
	}

	// Remove any whitespace
	cpu = strings.TrimSpace(cpu)

	// Handle millicores (e.g., "500m" = 0.5 cores)
	if strings.HasSuffix(cpu, "m") {
		milliStr := strings.TrimSuffix(cpu, "m")
		milli, err := strconv.ParseFloat(milliStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid CPU quantity: %s", cpu)
		}
		return milli / 1000, nil
	}

	// Parse as regular float
	cores, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU quantity: %s", cpu)
	}

	return cores, nil
}

// ParseMemory parses a memory/disk quantity string into bytes
func ParseMemory(memory string) (int64, error) {
	if memory == "" {
		return 0, nil
	}

	// Remove any whitespace
	memory = strings.TrimSpace(memory)

	// Extract numeric part and unit
	var numStr string
	var unit string

	for i, ch := range memory {
		if (ch >= '0' && ch <= '9') || ch == '.' {
			continue
		}
		numStr = memory[:i]
		unit = memory[i:]
		break
	}

	// If we went through the whole string without finding a unit,
	// it's just a number
	if numStr == "" && unit == "" {
		numStr = memory
	}

	if numStr == "" {
		return 0, fmt.Errorf("invalid memory quantity: %s", memory)
	}

	// If no unit specified, assume bytes
	if unit == "" {
		val, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory quantity: %s", memory)
		}
		return val, nil
	}

	// Parse the numeric value
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory quantity: %s", memory)
	}

	// Parse the unit and calculate bytes
	switch strings.ToLower(unit) {
	// Binary units
	case "ki", "kib":
		return int64(num * Kibibyte), nil
	case "mi", "mib":
		return int64(num * Mebibyte), nil
	case "gi", "gib":
		return int64(num * Gibibyte), nil
	case "ti", "tib":
		return int64(num * Tebibyte), nil

	// Decimal units
	case "k", "kb":
		return int64(num * Kilobyte), nil
	case "m", "mb":
		return int64(num * Megabyte), nil
	case "g", "gb":
		return int64(num * Gigabyte), nil
	case "t", "tb":
		return int64(num * Terabyte), nil

	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

// FormatBytes formats bytes into human-readable string
func FormatBytes(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	// Use binary units for consistency with Kubernetes
	units := []struct {
		size int64
		name string
	}{
		{Tebibyte, "Ti"},
		{Gibibyte, "Gi"},
		{Mebibyte, "Mi"},
		{Kibibyte, "Ki"},
	}

	for _, unit := range units {
		if bytes >= unit.size {
			value := float64(bytes) / float64(unit.size)
			if value == float64(int64(value)) {
				return fmt.Sprintf("%d%s", int64(value), unit.name)
			}
			return fmt.Sprintf("%.2f%s", value, unit.name)
		}
	}

	return fmt.Sprintf("%d", bytes)
}

// FormatCPU formats CPU cores into string
func FormatCPU(cores float64) string {
	if cores == 0 {
		return "0"
	}

	// If it's a whole number, format without decimal
	if cores == float64(int64(cores)) {
		return fmt.Sprintf("%d", int64(cores))
	}

	// Otherwise, format with appropriate precision
	return fmt.Sprintf("%.2f", cores)
}

// FormatCPUMillis formats CPU millicores into string
func FormatCPUMillis(millis int) string {
	if millis == 0 {
		return "0"
	}

	// If it's a whole number of cores, format as cores
	if millis%1000 == 0 {
		return fmt.Sprintf("%d", millis/1000)
	}

	// Otherwise, format as decimal cores
	return fmt.Sprintf("%.3g", float64(millis)/1000)
}

// ToResourcesConfig converts internal Resources to ResourcesConfig for display
func (r *Resources) ToResourcesConfig() *ResourcesConfig {
	if r == nil {
		return nil
	}

	config := &ResourcesConfig{}

	// Set requests if any are non-zero
	if r.CPURequestMillis > 0 || r.MemoryRequestBytes > 0 {
		config.Requests = &ResourceQuantities{}
		if r.CPURequestMillis > 0 {
			config.Requests.CPU = FormatCPUMillis(r.CPURequestMillis)
		}
		if r.MemoryRequestBytes > 0 {
			config.Requests.Memory = FormatBytes(r.MemoryRequestBytes)
		}
	}

	// Set limits if any are non-zero
	if r.CPULimitMillis > 0 || r.MemoryLimitBytes > 0 {
		config.Limits = &ResourceQuantities{}
		if r.CPULimitMillis > 0 {
			config.Limits.CPU = FormatCPUMillis(r.CPULimitMillis)
		}
		if r.MemoryLimitBytes > 0 {
			config.Limits.Memory = FormatBytes(r.MemoryLimitBytes)
		}
	}

	return config
}
