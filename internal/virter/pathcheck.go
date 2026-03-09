package virter

import (
	"fmt"
	"path/filepath"
	"strings"
)

// splitGlobPrefix splits a glob pattern into the longest path prefix that
// contains no glob metacharacters and the remaining suffix. This is similar to
// how Docker/BuildKit's splitWildcards works for COPY source validation.
// For example, "/home/user/*.txt" returns ("/home/user", "*.txt") and
// "foo/*/bar.txt" returns ("foo", "*/bar.txt").
func splitGlobPrefix(pattern string) (prefix, suffix string) {
	parts := strings.Split(filepath.ToSlash(pattern), "/")
	var prefixParts []string
	for i, p := range parts {
		if strings.ContainsAny(p, "*?[") {
			suffix = strings.Join(parts[i:], string(filepath.Separator))
			break
		}
		prefixParts = append(prefixParts, p)
	}
	prefix = strings.Join(prefixParts, string(filepath.Separator))
	if prefix == "" {
		prefix = "."
	}
	return prefix, suffix
}

// checkPathInWorkDir checks that the given path, after resolving to an absolute
// path, is within the current working directory. This prevents provisioning
// steps from accessing arbitrary host paths, analogous to how Docker restricts
// COPY/ADD to the build context.
func checkPathInWorkDir(path string, workDir string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to determine absolute path of %q: %w", path, err)
	}

	// Resolve symlinks so that both paths are compared on the same basis.
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}

	resolvedWorkDir := workDir
	if resolved, err := filepath.EvalSymlinks(workDir); err == nil {
		resolvedWorkDir = resolved
	}

	// Ensure workDir has a trailing separator so that e.g. "/home/foobar"
	// is not considered to be within "/home/foo".
	prefix := filepath.Clean(resolvedWorkDir) + string(filepath.Separator)
	if !strings.HasPrefix(absPath+string(filepath.Separator), prefix) {
		return fmt.Errorf("path %q resolves to %q, which is outside the working directory %q", path, absPath, resolvedWorkDir)
	}

	return nil
}
