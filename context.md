# mattermost-plugin-remind — Project Context

## Known Behavior / Gotchas

### Same-weekday scheduling pushes to next week if the time has passed
`server/occurrence.go::everyEN` (~line 1029): registering `every monday at 5:30pm`
on Monday after 17:30 schedules the first occurrence for next Monday. This is
intentional. To verify behavior live on the same day, register before the target
time, or test with a near-future time first (`in 5 minutes`).

### Bare `HH:MM` is 24-hour (since commit fixing `normalizeTime`)
`09:00` → 9 AM, `17:30` → 5:30 PM, regardless of when the command was typed.
Earlier behavior (pre-fix) inherited the registrar's current AM/PM from
`time.Now()`, so `09:00` could resolve to 21:00 if registered after noon.

### Channel target must be the URL handle, not the display name
`reminder.go::TriggerRemindersForTick` calls `GetChannelByName` with the literal
`~target` string. Mattermost channel URL handles are lowercase ASCII; non-ASCII
display names like "AI센터" do not resolve unless the underlying URL handle is
also that string (it usually isn't). Verify the handle in channel settings.

### `~channel` requires the bot to join
`reminder.go::ensureBotChannelMember` adds `remindbot` to the channel before the
first post. Private channels need permission for the bot to join; if the join
fails the registrar receives a DM with the failure reason.

### Cluster (HA) — occurrences are pinned to the registering host
`Occurrence.Hostname` is captured at registration time and the trigger loop in
`reminder.go` skips occurrences whose hostname doesn't match the local node. In
a multi-node deployment, the registering node must remain available, or
occurrences won't fire until rescheduled.

## Build Notes (Windows)

### Use `build/package.ps1`
Cross-builds for 5 platforms (linux/darwin amd64+arm64, windows amd64) and
bundles the tarball with explicit POSIX permissions (0755 for executables,
0644 for data) using Python's `tarfile` module — bypassing NTFS's lack of
exec-bit support.

### Why bare `tar` fails on Windows
NTFS does not preserve POSIX exec bits, so `chmod 755` followed by plain
`tar -czf` ships Linux/Darwin binaries as `-rw-r--r--` and Mattermost rejects
the plugin with a startup permission error. If `package.ps1` is unavailable,
force perms in the archive directly:

```
tar --mode='u=rwX,go=rX,a+x' -czf dist/<bundle>.tar.gz <plugin-id>/
```

### `make` is not assumed on Windows
The Makefile is the canonical build path on Linux/macOS. On Windows, prefer
`build/package.ps1`.
