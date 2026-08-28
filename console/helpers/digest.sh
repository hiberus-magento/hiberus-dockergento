#!/usr/bin/env bash

#
# A content digest that exists on both platforms.
#
# macOS has `shasum` and no `sha256sum`; Linux and Alpine have `sha256sum`. Code that reached for
# one of them recorded an empty checksum on the other half of the machines, which is the same as
# recording nothing while looking like it recorded something.
#
hm_digest() {
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 | cut -d' ' -f1
    elif command -v sha256sum >/dev/null 2>&1; then
        sha256sum | cut -d' ' -f1
    else
        # Not a cryptographic hash, but this compares two copies of a file; it is not a signature
        cksum | cut -d' ' -f1
    fi
}

#
# The digest of a file, or of a directory's contents.
#
# The names go into the digest alongside the contents: two skills with the same text under
# different file names are not the same skill, and a file appearing or disappearing has to change
# the result.
#
hm_digest_path() {
    local target="$1"

    if [ -d "$target" ]; then
        find "$target" -type f 2>/dev/null | LC_ALL=C sort | while IFS= read -r file; do
            printf '%s\n' "${file#"$target"/}"
            cat "$file"
        done | hm_digest
    elif [ -f "$target" ]; then
        hm_digest < "$target"
    else
        printf ''
    fi
}
