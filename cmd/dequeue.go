// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"log"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/cobra"
)

func dequeueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dequeue /path/to/spec.yaml",
		Short: "dequeues the DAG",
		Long:  `dagu dequeue /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Configuration load failed: %v", err)
			}
			dataStore := newDataStores(cfg)
			historyStore := dataStore.HistoryStore()
			queueStore := newQueueStore(cfg)

			found, err := queueStore.FindJobId(args[0])
			if err := historyStore.RemoveEmptyQueue(args[0]); err != nil {
				log.Fatal("Queue History data clean up failed", "error", err)
			}
			if found {
				log.Print("job id dequeued ", args[0])
			} else {
				log.Print(args[0], " is not present in the queue.")
			}
		},
	}
	return cmd
}
