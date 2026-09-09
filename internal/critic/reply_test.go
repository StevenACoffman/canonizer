package critic_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/canonizer/internal/critic"
	"github.com/StevenACoffman/skillet/finding"
)

func TestParseReply(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		reply          string
		wantErr        bool
		wantDiagnostic int
		wantUnexamined int
	}{
		"findings only": {
			reply: `{"diagnostics":[{"severity":"error","category":"unsupported",
				"path":"§1.1","message":"invented"}]}`,
			wantDiagnostic: 1, wantUnexamined: 0,
		},
		"findings with a coverage record": {
			reply: `{"diagnostics":[],"unexamined":[
				{"aspect":"the source's §7 examples","reason":"read the prose only"}]}`,
			wantDiagnostic: 0, wantUnexamined: 1,
		},
		"an empty reply is valid and says nothing": {
			reply:          `{"diagnostics":[]}`,
			wantDiagnostic: 0, wantUnexamined: 0,
		},
		// The whole reply goes, not just the entry: dropping it would turn "said nothing"
		// into "found nothing", which is the ambiguity the field exists to remove.
		"an entry with no reason rejects the whole reply": {
			reply: `{"diagnostics":[{"severity":"error","category":"unsupported",
				"path":"§1.1","message":"invented"}],
				"unexamined":[{"aspect":"§7"}]}`,
			wantErr: true,
		},
		"an entry with no aspect rejects the whole reply": {
			reply:   `{"diagnostics":[],"unexamined":[{"reason":"ran short of time"}]}`,
			wantErr: true,
		},
		"a whitespace-only reason is no reason": {
			reply:   `{"diagnostics":[],"unexamined":[{"aspect":"§7","reason":"   "}]}`,
			wantErr: true,
		},
		"malformed JSON is an error": {
			reply:   `{"diagnostics":[`,
			wantErr: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := critic.ParseReply([]byte(tc.reply))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseReply = %+v, want an error", got)
				}
				return
			}
			assertCounts(t, got, err, tc.wantDiagnostic, tc.wantUnexamined)
		})
	}
}

// TestParseReplyRejectionNamesTheEntry keeps the error actionable: a critic reply can carry
// several entries and "one of them is wrong" is not a repair instruction.
func TestParseReplyRejectionNamesTheEntry(t *testing.T) {
	t.Parallel()
	const reply = `{"diagnostics":[],"unexamined":[
		{"aspect":"§7","reason":"read the prose only"},
		{"aspect":"§9"}]}`
	_, err := critic.ParseReply([]byte(reply))
	if err == nil {
		t.Fatal("ParseReply accepted an invalid entry")
	}
	if !strings.Contains(err.Error(), "entry 1") {
		t.Errorf("error = %v, want it to name which entry failed", err)
	}
}

// assertCounts checks a successful parse produced the expected list lengths. Extracted to
// keep TestParseReply's table flat: the error branch plus three assertions inside a subtest
// is what pushes it over the complexity cap.
func assertCounts(t *testing.T, got finding.Result, err error, wantDiag, wantUnex int) {
	t.Helper()
	if err != nil {
		t.Fatalf("ParseReply: %v", err)
	}
	if len(got.Diagnostics) != wantDiag {
		t.Errorf("diagnostics = %d, want %d", len(got.Diagnostics), wantDiag)
	}
	if len(got.Unexamined) != wantUnex {
		t.Errorf("unexamined = %d, want %d", len(got.Unexamined), wantUnex)
	}
}
