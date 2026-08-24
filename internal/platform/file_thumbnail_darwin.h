#ifndef SAILBOARD_FILE_THUMBNAIL_DARWIN_H
#define SAILBOARD_FILE_THUMBNAIL_DARWIN_H

// Returns the OS's per-type/per-folder icon for path (any file or folder — NSWorkspace's
// iconForFile: works generically, not just for executables), downscaled to size x size and
// re-encoded as PNG into malloc'd memory the caller frees with sb_free (clipboard_darwin.h). *ok
// is 0 if path has no icon or something else went wrong.
void sb_file_icon(const char *path, int size, unsigned char **outData, long *outLen, int *ok);

#endif
