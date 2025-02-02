// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/cerbos/cerbos/client"
	"github.com/cerbos/cerbos/cmd/cerbosctl/get/internal/flagset"
	"github.com/cerbos/cerbos/cmd/cerbosctl/get/internal/printer"
	"github.com/cerbos/cerbos/cmd/cerbosctl/internal"
)

func List(c client.AdminClient, cmd *cobra.Command, format *flagset.Format) error {
	schemaIds, err := c.ListSchemas(context.Background())
	if err != nil {
		return fmt.Errorf("error while requesting schemas: %w", err)
	}

	tw := printer.NewTableWriter(cmd.OutOrStdout())
	if !format.NoHeaders {
		tw.SetHeader([]string{"SCHEMA ID"})
	}

	sort.Strings(schemaIds)
	for _, id := range schemaIds {
		tw.Append([]string{id})
	}
	tw.Render()

	return nil
}

func Get(c client.AdminClient, cmd *cobra.Command, format *flagset.Format, ids ...string) error {
	for idx := range ids {
		if idx%internal.MaxIDPerReq == 0 {
			schemas, err := c.GetSchema(context.Background(), ids[idx:internal.MinInt(idx+internal.MaxIDPerReq, len(ids)-idx)]...)
			if err != nil {
				return fmt.Errorf("error while requesting schema: %w", err)
			}

			if err = printSchema(cmd.OutOrStdout(), schemas, format.Output); err != nil {
				return fmt.Errorf("could not print schemas: %w", err)
			}
		}
	}
	return nil
}
