# Support

This is a Stage 1 scaffold. Before reporting an issue:

1. Confirm the build steps in the README.
2. Run `ctest --preset dev-debug`.
3. Check the log with `--debug`.
4. Include your Plasma, KWin, Qt, and KF6 versions.

Known limitation: automatic selected-text retrieval on Wayland is not implemented.
The application reports `SelectionUnavailable` instead of reading the clipboard.

When reporting a lookup problem, include whether the database path exists, whether the
default database or an override was used, and the exact command. Do not paste private
selected text.
