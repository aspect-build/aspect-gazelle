A rule kind whose `From` label does not parse, on a kind that is actually
generated — the case `rule-kinds` deliberately leaves out by never using its
bad kind.

The goldens record today's behaviour, which is wrong: `ApparentLoads` drops the
kind's `load()`, the rule call is still written, and gazelle exits 0. Bazel
then fails on the generated BUILD file with "name 'bad_from' is not defined".
Pinned here so a fix shows up as a golden diff.
