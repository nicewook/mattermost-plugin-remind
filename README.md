# Flexing Remind

_A Mattermost server plugin that schedules reminders for users and channels._

<img src="remind.png">

## Attribution

This plugin is forked from [scottleedavis/mattermost-plugin-remind](https://github.com/scottleedavis/mattermost-plugin-remind).

The original project is licensed under the Apache License 2.0. This fork keeps the original license and attribution while carrying Flexing-specific maintenance, dependency updates, and compatibility fixes.

## Installation

_Requires Mattermost Server v6.5.2 or greater._

1. Download the release bundle for your Mattermost server.
2. Upload the `.tar.gz` file in `System Console > Plugins > Management`.
3. Enable the plugin.
4. For a better cross-timezone experience, enable timezone support if your Mattermost deployment requires it.
5. If your server restricts cross-team direct messages, add `remindbot` to any team that needs to use the plugin.

## Usage

* `/remind` - opens an interactive dialog to schedule a reminder
* `/remind help` - displays help examples
* `/remind list` - displays a list of reminders
* `/remind [who] [what] [when]`
  * `/remind [who] [what] in [# (seconds|minutes|hours|days|weeks|months|years)]`
  * `/remind [who] [what] at [(noon|midnight|one..twelve|00:00am/pm|0000)] (every) [day|date]`
  * `/remind [who] [what] (on) [(monday-sunday|month&day|m/d/y|d.m.y)] (at) [time]`
  * `/remind [who] [what] every (other) [monday,...,sunday|weekdays|month&day|m/d|d.m] (at) [time]`
* `/remind [who] [when] [what]`

## Build

```sh
go build ./...
```

Build the plugin executables and package them into a Mattermost upload bundle:

```sh
make dist
```

The final bundle is:

```text
dist/ai.flexing.mattermost-plugin-remind-1.0.0.tar.gz
```
