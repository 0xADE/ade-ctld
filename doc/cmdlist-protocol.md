# CMDLIST Protocol

All `ade-*-ctld` servers should follow this protocol.

It supports binary or text representation. Well, text format only really supported yet.

**General for both formats:**
| Bytes numbers (+len) | Data           | Description                 |
|----------------------|----------------|-----------------------------|
| 0-2 (3)              | "TXT" or "BIN" | Text or binary format used  |

**Text format:**
| Byte number (+len) | Example | Description               |
|--------------------|---------|---------------------------|
| 3-4 (2)            | "01"    | Protocol version in ASCII |

**Binary format:**
| Byte number(+len) | Example | Description                |
|-------------------|---------|----------------------------|
| 3 (1)             | 01      | Protocol version as binary |

## Commands

### +filter-name
*Arguments:* Arbitrary number of arguments of types <str> or <bool>
*Sets filename filter by which applications are searched in PATH. Both the direct filename and its headers from desktop files are considered in the name. String arguments are treated as search terms, while boolean arguments control the logical operation (OR, AND, NOT) for combining multiple search terms.*
*Returns:* cmd: +filter-name, status: 0

### +filter-cat
*Arguments:* Arbitrary number of <str> arguments and optional <bool> arguments
*Add arguments from string parameters as category filters. By default, multiple categories are combined with AND logic, unless OR boolean argument is explicitly provided.*
*Returns:* cmd: +filter-cat, status: 0

### +filter-path
*Arguments:* Arbitrary number of <str> arguments and optional <bool> arguments
*Add arguments from string parameters as path filters. By default, multiple paths are combined with OR logic, unless AND boolean argument is explicitly provided.*
*Returns:* cmd: +filter-path, status: 0

### 0filters
*Arguments:* None
*Reset all filters (name, category, and path filters) to empty state.*
*Returns:* cmd: 0filters, status: 0

### list
*Arguments:* None
*Return list of application names (with their IDs) according to current filter set. Different filter types (name, category, path) are combined with OR logic.*
*Returns:* len: <total_count>, limited: <displayed_count> (if limited), offset: <offset> (if paginated), list-next: <next_offset> <limit> (if more items available), followed by body containing ID-name pairs

### list-next
*Arguments:* offset <int> (required), limit <int> (optional)
*Return next portion of entries from the current filter set starting from the specified offset. If limit is not provided, uses the default list limit from configuration.*
*Returns:* len: <total_count>, limited: <displayed_count>, offset: <current_offset>, list-next: <next_offset> <limit> (if more items available), followed by body containing ID-name pairs

### run
*Arguments:* id <int> (required)
*Run application by ID from the index database. The application is executed either directly or in a terminal if specified in its desktop entry.*
*Returns:* cmd: run, idx: <application_id>, status: <execution_status>, pid: <process_id>

### lang
*Arguments:* isolang <str> (required)
*Set preferred language for returning localized results (for example, when selecting localizations returned from desktop files).*
*Returns:* cmd: lang, status: 0, lang: <language_code>

## Fort Style

Uses reverse Polish notation for commands and arguments.

- Any argument followed by 0A (ASCII LF)
- Any string argument prefixed with ", type <str>
- Comment lines started with # are ignored
- Empty commands (consecutive 0A) are ignored and reflected in the listing as blank lines
- Logical operators or, and, not (these are not strings, passed without prefix "), type <bool>
- Numbers are also passed without " (type <int>)

### Examples

Session example:
```
# Add data to path list:
"~/bin
"~/apps
+path

# Save current settings to file config: (TODO: Not implemented)
saveconf

# Set filtering by program names substring "fi fox" (e.g. firefox)
"fi fox
+filter-name

# Add filter for categories graphics AND viewers (otherwise OR is assumed by default)
"graphics
"viewers
and
+filter-cat

# Return list of names (headers for UI list + id) matching current filter
list

# Return next portion from filter
list-next
```

```
#log log this line, example comment with pragma (word after # without space)
```

Commands are processed as they become ready. Arguments are simply pushed onto the stack one by one until a command comes for them. If arguments are valid for the command, it executes and sends the result to the socket. Otherwise, an error with a description of the problem is sent to the socket.

## Command Results

Result of any command is returned as:

```
<attrs block>
[body block]
```

The attrs block can be considered as headers. These are attributes in the form <key> <value>, where there is one separator (space) between key and value.
After value, LF (0A) is mandatory. The block ends with two consecutive LF, after which the body block optionally follows.

Examples of response to list command:

```
len: 2
pages: 1

1235 Firefox
1262 Firefox (Wayland)
```

Then after request from UI:
```
1262
run
```

Firefox for Wayland will be launched and the launch status and process presence check will be returned:

```
cmd: run
idx: 1262
status: 0
pid: 2365
```

Here is an example of response to erroneous request:

```
0
run
```

Reply:
```
error-cmd: run
error: index not found
desc: Can't run application, requested index not found.
```

Body is empty here, because of error.
