// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	auditv1 "github.com/cerbos/cerbos/api/genpb/cerbos/audit/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"github.com/cerbos/cerbos/client/testutil"
	"github.com/cerbos/cerbos/internal/test"
)

func TestCollectLogs(t *testing.T) {
	t.Run("access logs", func(t *testing.T) {
		receiver := func() (*responsev1.ListAuditLogEntriesResponse, error) {
			return &responsev1.ListAuditLogEntriesResponse{Entry: &responsev1.ListAuditLogEntriesResponse_AccessLogEntry{
				AccessLogEntry: &auditv1.AccessLogEntry{CallId: "test"},
			}}, nil
		}

		logs, err := collectLogs(receiver)
		require.NoError(t, err)

		log := <-logs
		require.Equal(t, "test", log.accessLog.CallId)
		require.Empty(t, logs)
	})

	t.Run("return io.EOF directly", func(t *testing.T) {
		receiver := func() (*responsev1.ListAuditLogEntriesResponse, error) {
			return nil, io.EOF
		}

		logs, err := collectLogs(receiver)
		require.NoError(t, err)
		require.Empty(t, logs)
	})

	t.Run("error from receiver", func(t *testing.T) {
		receiver := func() (*responsev1.ListAuditLogEntriesResponse, error) { return nil, errors.New("test-error") }

		logs, err := collectLogs(receiver)
		require.NoError(t, err)

		log := <-logs
		al, err := log.AccessLog()
		require.Nil(t, al)
		require.Error(t, err)
	})
}

func TestAuditLogs(t *testing.T) {
	t.Run("should fail on invalid log options", func(t *testing.T) {
		c := GrpcAdminClient{client: svcv1.NewCerbosAdminServiceClient(&grpc.ClientConn{})}

		_, err := c.AuditLogs(context.Background(), AuditLogOptions{
			Type: AccessLogs,
			Tail: 10000,
		})

		require.Error(t, err)
	})

	t.Run("should fail if log type is different", func(t *testing.T) {
		c := GrpcAdminClient{client: svcv1.NewCerbosAdminServiceClient(&grpc.ClientConn{})}

		_, err := c.AuditLogs(context.Background(), AuditLogOptions{
			Type: AuditLogType(100),
			Tail: 10000,
		})

		require.Error(t, err)
	})
}

func TestListPolicies(t *testing.T) {
	const (
		adminUsername = "cerbos"
		adminPassword = "cerbosAdmin"
		timeout       = 15 * time.Second
	)

	serverOpts := []testutil.ServerOpt{
		testutil.WithPolicyRepositoryDatabase("sqlite3", fmt.Sprintf("%s?_fk=true", filepath.Join(t.TempDir(), "cerbos.db"))),
		testutil.WithAdminAPI(adminUsername, adminPassword),
	}

	s, err := testutil.StartCerbosServer(serverOpts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = s.Stop()
	})

	ac, err := NewAdminClientWithCredentials(s.GRPCAddr(), adminUsername, adminPassword, WithPlaintext(), WithConnectTimeout(timeout))
	require.NoError(t, err)

	ps := NewPolicySet()
	err = test.FindPolicyFiles(t, "store", func(path string) error {
		ps.AddPolicyFromFile(path)
		return ps.Err()
	})

	require.NoError(t, err)
	require.NoError(t, ac.AddOrUpdatePolicy(context.Background(), ps))

	t.Run("should get the list of policies", func(t *testing.T) {
		policies, err := ac.ListPolicies(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, policies)
	})

	t.Run("policy metadata should include store identifier", func(t *testing.T) {
		testCases := []string{"derived_roles.alpha", "principal.donald_duck.20210210", "resource.leave_request.20210210"}

		for _, testCase := range testCases {
			policies, err := ac.GetPolicy(context.Background(), testCase)
			require.NoError(t, err)
			require.Len(t, policies, 1)
			require.Equal(t, testCase, policies[0].Metadata.StoreIdentifer)
		}
	})
}
