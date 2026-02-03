#!/bin/bash
# Record demo GIFs for the vibed README using vhs (Charm's terminal recorder)
#
# Prerequisites:
#   brew install vhs
#   cargo install vibed (or have vibed in PATH)
#
# Usage:
#   ./docs/scripts/record-readme-demos.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="$REPO_ROOT/docs/images"

# Use globally installed vibed
GV="vibed"

# Demo repo with multiple worktrees - use a worktree with interesting diff content
DEMO_REPO="$HOME/conductor/workspaces/cria/luxembourg"

mkdir -p "$OUTPUT_DIR"

echo ""
echo "Recording README demos..."
echo "Output: $OUTPUT_DIR"
echo "Demo repo: $DEMO_REPO"
echo ""

# Demo 1: Navigation - scroll through diff
echo "==> Recording: vibed-navigation.gif"
cat > /tmp/vibed-navigation.tape << TAPE
Output "${OUTPUT_DIR}/vibed-navigation.gif"
Set Shell "bash"
Set FontSize 12
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set TypingSpeed 50ms
Set Padding 10

Hide
Sleep 100ms
Type "cd ${DEMO_REPO} && ${GV}"
Enter
Sleep 3s
Show

Sleep 300ms

# Scroll down
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 400ms

# Page down
Ctrl+d
Sleep 400ms
Ctrl+d
Sleep 400ms

# Next file
Type "n"
Sleep 400ms
Type "n"
Sleep 400ms

# Back to top
Type "g"
Sleep 600ms

# Quit
Type "q"
Sleep 200ms
TAPE
vhs /tmp/vibed-navigation.tape
echo "Done: $OUTPUT_DIR/vibed-navigation.gif"
echo ""

# Demo 2: Toggle unified/side-by-side view
echo "==> Recording: vibed-views.gif"
cat > /tmp/vibed-views.tape << TAPE
Output "${OUTPUT_DIR}/vibed-views.gif"
Set Shell "bash"
Set FontSize 12
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 10

Hide
Type "cd ${DEMO_REPO} && ${GV}"
Enter
Sleep 1s
Show

Sleep 500ms

# Toggle to unified view
Type "u"
Sleep 1200ms

# Toggle back to side-by-side
Type "u"
Sleep 1000ms

# Quit
Type "q"
Sleep 200ms
TAPE
vhs /tmp/vibed-views.tape
echo "Done: $OUTPUT_DIR/vibed-views.gif"
echo ""

# Demo 3: Worktree switching
echo "==> Recording: vibed-worktree.gif"
cat > /tmp/vibed-worktree.tape << TAPE
Output "${OUTPUT_DIR}/vibed-worktree.gif"
Set Shell "bash"
Set FontSize 12
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 10

Hide
Sleep 100ms
Type "cd ${DEMO_REPO} && ${GV}"
Enter
Sleep 3s
Show

Sleep 300ms

# Open worktree selector
Type "w"
Sleep 800ms

# Move down to select another worktree
Type "j"
Sleep 300ms

# Select it
Enter
Sleep 1000ms

# Quit
Type "q"
Sleep 200ms
TAPE
vhs /tmp/vibed-worktree.tape
echo "Done: $OUTPUT_DIR/vibed-worktree.gif"
echo ""

# Demo 4: Collapse/expand files
echo "==> Recording: vibed-collapse.gif"
cat > /tmp/vibed-collapse.tape << TAPE
Output "${OUTPUT_DIR}/vibed-collapse.gif"
Set Shell "bash"
Set FontSize 12
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 10

Hide
Type "cd ${DEMO_REPO} && ${GV}"
Enter
Sleep 1s
Show

Sleep 500ms

# Collapse current file
Space
Sleep 400ms

# Move down
Type "j"
Sleep 200ms

# Collapse that file
Space
Sleep 400ms

# Collapse all
Type "z"
Sleep 600ms

# Expand all
Type "z"
Sleep 600ms

# Quit
Type "q"
Sleep 200ms
TAPE
vhs /tmp/vibed-collapse.tape
echo "Done: $OUTPUT_DIR/vibed-collapse.gif"
echo ""

# Demo 5: Help screen
echo "==> Recording: vibed-help.gif"
cat > /tmp/vibed-help.tape << TAPE
Output "${OUTPUT_DIR}/vibed-help.gif"
Set Shell "bash"
Set FontSize 12
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Padding 10

Hide
Type "cd ${DEMO_REPO} && ${GV}"
Enter
Sleep 1s
Show

Sleep 500ms

# Show help
Type "?"
Sleep 2s

# Close help
Type "?"
Sleep 500ms

# Quit
Type "q"
Sleep 200ms
TAPE
vhs /tmp/vibed-help.tape
echo "Done: $OUTPUT_DIR/vibed-help.gif"
echo ""

# Cleanup tape files
rm -f /tmp/vibed-*.tape

echo ""
echo "All demos recorded!"
ls -la "$OUTPUT_DIR"/*.gif 2>/dev/null || echo "No GIFs found"
