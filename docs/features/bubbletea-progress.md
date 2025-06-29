# Bubble Tea Progress Display

Dagu now includes an experimental Bubble Tea-based progress display that provides a smoother, more modern terminal UI experience during DAG execution.

## Overview

The new progress display is built using the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework, which offers:

- Smoother animations and updates
- Better terminal handling
- More consistent rendering across different terminal emulators
- Improved styling with lipgloss

## Enabling Bubble Tea Progress

To use the new progress display, set the environment variable:

```bash
export DAGU_USE_BUBBLETEA_PROGRESS=true
```

Then run your DAG as usual:

```bash
dagu start my-dag.yaml
```

## Features

The Bubble Tea progress display maintains all the features of the original display:

- **Real-time Progress**: Shows currently running steps with animated spinners
- **Progress Bar**: Visual representation of overall completion
- **Step Status**: Clear icons for success (✓), failure (✗), running (●), and queued (○)
- **Timing Information**: Shows elapsed time for each step and overall execution
- **Error Display**: Shows error messages for failed steps
- **Child DAG Support**: Displays information about nested DAG executions
- **Final Summary**: Shows all completed steps when execution finishes

## Visual Comparison

### Original Progress Display
- Uses raw terminal escape sequences
- Updates every 100ms with a ticker
- Basic color support

### Bubble Tea Progress Display
- Uses the Bubble Tea framework for rendering
- Smooth animations with built-in spinner component
- Enhanced styling with lipgloss
- Better terminal resize handling

## Implementation Details

The new implementation:
- Follows the same `ProgressReporter` interface as the original
- Maintains thread-safety through message passing instead of mutexes
- Uses Bubble Tea's Model-View-Update pattern
- Renders in alternate screen mode (fullscreen)

## Testing

To test the new progress display:

1. Use the example DAG:
   ```bash
   DAGU_USE_BUBBLETEA_PROGRESS=true dagu start example_bubbletea_progress.yaml
   ```

2. Run your existing DAGs with the environment variable set

3. Compare the experience with the original display

## Future Improvements

Potential enhancements for the Bubble Tea progress display:

- Interactive controls (pause/resume)
- Collapsible sections for better organization
- Color themes
- Progress history view
- Real-time log viewing
- Mouse support for navigation

## Troubleshooting

If you experience issues with the new display:

1. Ensure your terminal supports ANSI escape sequences
2. Try different terminal emulators (iTerm2, Terminal.app, etc.)
3. Check that the terminal size is adequate (minimum 80x24)
4. Disable the feature by unsetting the environment variable:
   ```bash
   unset DAGU_USE_BUBBLETEA_PROGRESS
   ```

## Feedback

This is an experimental feature. Please report any issues or suggestions for improvement.