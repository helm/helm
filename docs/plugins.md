# OCI Plugin Installation

## Symlink Support

OCI plugins may contain symbolic links. For security:

- Symlink targets must be relative paths
- Absolute paths are rejected
- Path traversal attempts are blocked
- On Windows, Administrator privileges or Developer Mode required
