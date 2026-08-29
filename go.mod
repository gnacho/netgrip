module github.com/gnacho/netgrip

go 1.25.0

require github.com/gnacho/netpulse/agent v0.0.0

// TODO: replace with a tagged version of github.com/gnacho/netpulse/agent
// before the public release. The absolute path points at today's local
// worktree for development builds only.
replace github.com/gnacho/netpulse/agent => /tmp/opencode/np-unlazy/C-agent/agent
