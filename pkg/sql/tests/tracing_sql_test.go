// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/stretchr/testify/require"
)

func TestSetTraceSpansVerbosityBuiltin(t *testing.T) {
	defer leaktest.AfterTest(t)()
	si, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer si.Stopper().Stop(context.Background())
	s := si.(*server.TestServer)
	r := sqlutils.MakeSQLRunner(db)

	tr := s.Tracer()

	// Try to toggle the verbosity of a trace that doesn't exist, returns false.
	// NB: Technically this could return true in the unlikely scenario that there
	// is a trace with ID of 0.
	r.CheckQueryResults(
		t,
		"SELECT * FROM crdb_internal.set_trace_verbose(0, true)",
		[][]string{{`false`}},
	)

	root := tr.StartSpan("root", tracing.WithForceRealSpan())
	defer root.Finish()
	require.False(t, root.IsVerbose())

	child := tr.StartSpan("root.child", tracing.WithParent(root))
	defer child.Finish()
	require.False(t, child.IsVerbose())

	childChild := tr.StartSpan("root.child.child", tracing.WithParent(child))
	defer childChild.Finish()
	require.False(t, childChild.IsVerbose())

	// Toggle the trace's verbosity and confirm all spans are verbose.
	traceID := root.TraceID()
	query := fmt.Sprintf(
		"SELECT * FROM crdb_internal.set_trace_verbose(%d, true)",
		traceID,
	)
	r.CheckQueryResults(
		t,
		query,
		[][]string{{`true`}},
	)

	require.True(t, root.IsVerbose())
	// The children have not been made verbose. This is an artifact of the current
	// implementation; it might change in the future. Children are only registered
	// with the parent if the parent is recording at the time when the children
	// are created (which was not the case here). set_trace_verbose only operates
	// on spans that are linked to their parent. set_trace_verbose could also go
	// through the span registry looking for more spans in the trace, but it
	// doesn't currently do that (and also currently we don't add most spans to
	// the registry, but that also should change in the future).
	require.False(t, child.IsVerbose())
	require.False(t, childChild.IsVerbose())

	// New child of verbose child span should also be verbose by default.
	newChild := tr.StartSpan("root.child.newchild", tracing.WithParent(root))
	defer newChild.Finish()
	require.True(t, newChild.IsVerbose())

	// Toggle the trace's verbosity and confirm none of the spans are verbose.
	query = fmt.Sprintf(
		"SELECT * FROM crdb_internal.set_trace_verbose(%d, false)",
		traceID,
	)
	r.CheckQueryResults(
		t,
		query,
		[][]string{{`true`}},
	)

	require.False(t, root.IsVerbose())
	require.False(t, child.IsVerbose())
	require.False(t, childChild.IsVerbose())
	require.False(t, newChild.IsVerbose())
}
