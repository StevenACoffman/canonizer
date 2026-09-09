package critic

import (
	"encoding/json"
	"strconv"

	"github.com/StevenACoffman/skillet/finding"
	errors "github.com/StevenACoffman/toerr/errors"
)

// ParseReply parses a cold-critic reply into a findings result.
//
// Parsing and validating are one call rather than two so a caller cannot do the first and
// forget the second. The reply is the only thing canonizer receives back from a grader --
// it emits a prompt and reads this -- so the shape check belongs beside the prompt that
// asked for it, not in whichever command happens to read the file.
//
// **A malformed coverage entry rejects the whole reply.** Dropping the bad entry and
// keeping the rest is the tempting behaviour and the wrong one: an `unexamined` list is
// the record of what the critic did not look at, so silently discarding half of it turns
// "said nothing" into "found nothing" -- exactly the ambiguity the field exists to remove.
// A reply that cannot state its gaps correctly has not earned the reading that its empty
// findings mean the ruleset is clean.
//
// finding.Unexamined.Valid is the predicate, and it requires both an aspect and a reason.
// An aspect with no reason is the boilerplate a required-but-unread field fills with: it
// satisfies the schema and records nothing.
//
// Requires: data is the raw reply bytes.
// Ensures:  on success every Unexamined entry is Valid; it is pure and reads no I/O.
func ParseReply(data []byte) (finding.Result, error) {
	var result finding.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return finding.Result{}, errors.WrapWithMessage(err, "critic: parse reply")
	}
	for i := range result.Unexamined {
		if !result.Unexamined[i].Valid() {
			return finding.Result{}, errors.New(
				"critic: reply rejected: unexamined entry " + strconv.Itoa(i) +
					" states no aspect or no reason; a coverage record that records " +
					"nothing makes an empty findings list unreadable")
		}
	}
	return result, nil
}
