package config

import (
    "strings"
    "testing"
    
    "github.com/spf13/viper"
    "github.com/stretchr/testify/require"
)

func TestQueueConfigLoading(t *testing.T) {
    yaml := `
queues:
  enabled: true
  config:
    - name: Default
      maxConcurrency: 3
    - name: HighLoad
      maxConcurrency: 2
`
    v := viper.New()
    v.SetConfigType("yaml")
    err := v.ReadConfig(strings.NewReader(yaml))
    require.NoError(t, err)

    loader := NewConfigLoader(v, WithService(ServiceScheduler))
    cfg, err := loader.Load()
    require.NoError(t, err)
    
    t.Logf("Queues Enabled: %v", cfg.Queues.Enabled)
    t.Logf("Queues Config length: %d", len(cfg.Queues.Config))
    for i, q := range cfg.Queues.Config {
        t.Logf("Config[%d]: Name=%s, MaxActiveRuns=%d", i, q.Name, q.MaxActiveRuns)
    }
    
    require.True(t, cfg.Queues.Enabled)
    require.Len(t, cfg.Queues.Config, 2)
    
    require.Equal(t, "Default", cfg.Queues.Config[0].Name)
    require.Equal(t, 3, cfg.Queues.Config[0].MaxActiveRuns, "Expected maxConcurrency=3 but got %d", cfg.Queues.Config[0].MaxActiveRuns)
    
    require.Equal(t, "HighLoad", cfg.Queues.Config[1].Name)
    require.Equal(t, 2, cfg.Queues.Config[1].MaxActiveRuns)
}
