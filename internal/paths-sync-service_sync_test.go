package internal

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestPathsSyncService_Sync_BasicFileSync(t *testing.T) {
	// Create temporary directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files in source
	testFiles := map[string]string{
		"file1.txt": "content of file1",
		"file2.txt": "content of file2",
		"file3.log": "log content",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup service
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced
	verifyFileSymlinks(t, dstDir, testFiles)
}

func TestPathsSyncService_Sync_WithFileGlob(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files - only .txt files should be synced
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"file3.log": "should not be synced",
		"file4.md":  "should not be synced",
	}
	setupFiles(t, srcDir, testFiles)

	expectedFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}

	// Setup service with glob pattern
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + ":*.txt:" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify .txt files were synced
	verifyFileSymlinks(t, dstDir, expectedFiles)

	// Verify .log and .md files were not synced
	verifyFilesNotExist(t, dstDir, []string{"file3.log", "file4.md"})
}

func TestPathsSyncService_Sync_WithOwner(t *testing.T) {
	if !isRunningInContainer() {
		t.Skip("Skipping test that requires container environment")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup service with specific ownership (UID 1000)
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir + ":1000")
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced
	verifyFileSymlinks(t, dstDir, testFiles)

	// Verify file permissions
	for fileName := range testFiles {
		veriyFileOwnerGroup(t, dstDir, fileName, 1000, -1)
	}
}

func TestPathsSyncService_Sync_WithGroup(t *testing.T) {
	if !isRunningInContainer() {
		t.Skip("Skipping test that requires container environment")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup service with specific ownership (GID 1000)
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir + "::1000")
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced
	verifyFileSymlinks(t, dstDir, testFiles)

	// Verify file permissions
	for fileName := range testFiles {
		veriyFileOwnerGroup(t, dstDir, fileName, -1, 1000)
	}
}

func TestPathsSyncService_Sync_WithMode(t *testing.T) {
	if !isRunningInContainer() {
		t.Skip("Skipping test that requires container environment")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup service with specific permissions
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir + ":::0640")
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced
	verifyFileSymlinks(t, dstDir, testFiles)

	// Verify file permissions
	for fileName := range testFiles {
		verifyFileMode(t, dstDir, fileName, 0640)
	}
}

func TestPathsSyncService_Sync_WithDefaultMode(t *testing.T) {
	if !isRunningInContainer() {
		t.Skip("Skipping test that requires container environment")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup service with specific permissions
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir + ":::0000")
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced
	verifyFileSymlinks(t, dstDir, testFiles)

	// Verify file permissions
	for fileName := range testFiles {
		verifyFileMode(t, dstDir, fileName, 0400)
		verifyDirMode(t, dstDir, fileName, 0511)
	}
}

func TestPathsSyncService_Sync_MergedDstPaths(t *testing.T) {
	srcDir1 := t.TempDir()
	srcDir2 := t.TempDir()
	dstDir := t.TempDir()

	// Create test files in both source directories
	testFiles1 := map[string]string{
		"config1.yaml": "config: value1",
		"shared.conf":  "shared content from dir1",
	}
	testFiles2 := map[string]string{
		"config2.yaml": "config: value2",
		"shared.conf":  "shared content from dir2", // This should override dir1
	}
	setupFiles(t, srcDir1, testFiles1)
	setupFiles(t, srcDir2, testFiles2)

	expectedFiles := map[string]string{
		"config1.yaml": "config: value1",
		"config2.yaml": "config: value2",
		"shared.conf":  "shared content from dir2", // Last one should win
	}

	// Setup service with multiple path configs
	service := setupTestService(t)
	pathConfig1, err := ParsePathConfig(srcDir1 + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config 1: %v", err)
	}
	pathConfig2, err := ParsePathConfig(srcDir2 + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config 2: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig1, pathConfig2}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced
	verifyFileSymlinks(t, dstDir, expectedFiles)
}

func TestPathsSyncService_Sync_StaleFileRemoval(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create initial files
	initialFiles := map[string]string{
		"keep.txt":   "keep this",
		"remove.txt": "remove this",
	}
	setupFiles(t, srcDir, initialFiles)
	expectedFiles := map[string]string{
		"keep.txt": "keep this",
	}

	// Setup service and perform initial sync
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// First sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Initial sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify initial files were synced
	verifyFileSymlinks(t, dstDir, initialFiles)

	// Remove one file from source
	err = os.Remove(filepath.Join(srcDir, "remove.txt"))
	if err != nil {
		t.Fatalf("Failed to remove source file: %v", err)
	}

	// Second sync
	service.mu.Lock()
	result = service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Second sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify remaining files
	verifyFileSymlinks(t, dstDir, expectedFiles)
}

func TestPathsSyncService_Sync_WithTemplateProcessing(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"template.txt": `Hello {{.Name}}!
Current time: {{now | date "2006-01-02"}}
`,
	}
	setupFiles(t, srcDir, testFiles)
	expectedFiles := map[string]string{
		"template.txt": `Hello World!
Current time: ` + time.Now().Format("2006-01-02") + `
`,
	}

	// Setup service with patch engine
	service := setupTestService(t)

	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.patchEngine.data = map[string]interface{}{
		"Name": "World",
	}
	service.mu.Unlock()

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform sync
	service.mu.Lock()
	result := service.sync()
	service.mu.Unlock()

	if !result {
		t.Fatal("Sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify data structure
	verifyDataStructure(t, dstDir)

	// Verify files were synced and processed
	verifyFileSymlinks(t, dstDir, expectedFiles)
}

func TestPathsSyncService_Sync_ErrorHandling(t *testing.T) {
	// Test with non-existent source directory
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig("/non/existent/path::/tmp/dst")
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	result := service.sync()
	service.mu.Unlock()

	if result {
		t.Error("Sync should fail with non-existent source directory")
	}
}

func TestPathsSyncService_Sync_DoubleSync(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup and sync twice to test symlink updates
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// First sync
	service.mu.Lock()
	result1 := service.sync()
	service.mu.Unlock()

	if !result1 {
		t.Fatal("First sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify initial symlink structure
	verifyDataStructure(t, dstDir)

	// Second sync (should update symlinks)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	service.mu.Lock()
	result2 := service.sync()
	service.mu.Unlock()

	if !result2 {
		t.Fatal("Second sync operation failed")
	}

	printDstStructure(t, dstDir)

	// Verify symlinks still work
	verifyFileSymlinks(t, dstDir, testFiles)
}

func TestPathsSyncService_Sync_DataDirectoryCleanup(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	}
	setupFiles(t, srcDir, testFiles)

	// Setup service
	service := setupTestService(t)
	pathConfig, err := ParsePathConfig(srcDir + "::" + dstDir)
	if err != nil {
		t.Fatalf("Failed to parse path config: %v", err)
	}

	service.mu.Lock()
	service.pathConfigs = []*PathConfig{pathConfig}
	service.mu.Unlock()

	// Perform first sync
	service.mu.Lock()
	result1 := service.sync()
	service.mu.Unlock()

	if !result1 {
		t.Fatal("First sync operation failed")
	}

	// Count data directories
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatalf("Failed to read destination directory: %v", err)
	}

	var dataDirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "..") {
			dataDirs = append(dataDirs, entry.Name())
		}
	}

	if len(dataDirs) != 1 {
		t.Errorf("Expected 1 data directory, got %d", len(dataDirs))
	}

	// Perform second sync
	time.Sleep(2 * time.Millisecond) // Ensure different timestamp
	service.mu.Lock()
	result2 := service.sync()
	service.mu.Unlock()

	if !result2 {
		t.Fatal("Second sync operation failed")
	}

	// Count data directories again - old ones should be cleaned up
	entries, err = os.ReadDir(dstDir)
	if err != nil {
		t.Fatalf("Failed to read destination directory: %v", err)
	}

	var newDataDirs []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "..") {
			newDataDirs = append(newDataDirs, entry.Name())
		}
	}

	if len(newDataDirs) != 1 {
		t.Errorf("Expected 1 data directory after cleanup, got %d", len(newDataDirs))
	}

	// Verify old data directory was removed
	if len(newDataDirs) > 0 && len(dataDirs) > 0 && newDataDirs[0] == dataDirs[0] {
		t.Error("Old data directory should have been replaced")
	}
}

// Helper functions

func setupTestService(t *testing.T) *PathsSyncService {
	// Use a test logger that doesn't spam output
	logger := zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel))
	zap.ReplaceGlobals(logger)

	service, err := NewPathsSyncService(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create test service: %v", err)
	}
	return service
}

func setupFiles(t *testing.T, dir string, files map[string]string) {
	for fileName, content := range files {
		err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}
}

func verifyDataStructure(t *testing.T, dstDir string) {
	// Verify that the ..data symlink exists
	dataLinkPath := filepath.Join(dstDir, "..data")
	if _, err := os.Stat(dataLinkPath); err != nil {
		t.Errorf("%q symlink should exist in %q: %v", "..data", dstDir, err)
		return
	}

	// Verify it's a symlink
	fileInfo, err := os.Lstat(dataLinkPath)
	if err != nil {
		t.Errorf("Failed to get %q link info in %q: %v", "..data", dstDir, err)
		return
	}

	if fileInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("%q in %q should be a symlink", "..data", dstDir)
		return
	}

	// Verify the symlink target exists
	target, err := os.Readlink(dataLinkPath)
	if err != nil {
		t.Errorf("Failed to read %q symlink in %q: %v", "..data", dstDir, err)
		return
	}

	targetPath := filepath.Join(dstDir, target)
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("Target %q of %q symlink in %q should exist: %v", target, "..data", dstDir, err)
		return
	}
}

func verifyFileSymlinks(t *testing.T, dstDir string, expectedFiles map[string]string) {
	// Verify that symlinks point to the right content
	for fileName, expectedContent := range expectedFiles {
		filePath := filepath.Join(dstDir, fileName)

		// Check if it's a symlink
		fileInfo, err := os.Lstat(filePath)
		if err != nil {
			t.Errorf("Failed to get file info for %q: %v", fileName, err)
			continue
		}

		if fileInfo.Mode()&os.ModeSymlink == 0 {
			t.Errorf("File %q should be a symlink", fileName)
			continue
		}

		// Read content through symlink
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read symlink %q: %v", fileName, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("Symlink %q has incorrect content. Expected: %q, Got: %q",
				fileName, expectedContent, string(content))
		}
	}
}

func verifyFilesNotExist(t *testing.T, dstDir string, files []string) {
	for _, fileName := range files {
		filePath := filepath.Join(dstDir, fileName)
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			if err != nil {
				t.Errorf("Failed to check file %q: %v", fileName, err)
			} else {
				t.Errorf("File %q should not exist in %q", fileName, dstDir)
			}
		}
	}
}

func verifyFileMode(t *testing.T, dstDir, fileName string, expectedMode os.FileMode) {
	filePath, err := filepath.EvalSymlinks(filepath.Join(dstDir, fileName))
	if err != nil {
		t.Errorf("Failed to resolve symlink for %s: %v", fileName, err)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Errorf("Failed to get file info for %s: %v", fileName, err)
		return
	}

	actualMode := fileInfo.Mode() & os.ModePerm
	if actualMode != expectedMode {
		t.Errorf("File %q in %q has incorrect mode. Expected: %04o, Got: %04o",
			fileName, dstDir, expectedMode, actualMode)
	}
}

func verifyDirMode(t *testing.T, dstDir, fileName string, expectedMode os.FileMode) {
	filePath, err := filepath.EvalSymlinks(filepath.Join(dstDir, fileName))
	if err != nil {
		t.Errorf("Failed to resolve symlink for %s: %v", fileName, err)
		return
	}

	dirPath := filepath.Dir(filePath)
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Errorf("Failed to get directory info for %q: %v", dirPath, err)
		return
	}

	actualMode := fileInfo.Mode() & os.ModePerm
	if actualMode != expectedMode {
		t.Errorf("File %q in %q has incorrect mode. Expected: %04o, Got: %04o",
			fileName, dstDir, expectedMode, actualMode)
	}
}

func veriyFileOwnerGroup(t *testing.T, dstDir, fileName string, expectedUID, expectedGID int64) {
	filePath := filepath.Join(dstDir, fileName)
	var fileInfo os.FileInfo
	var err error

	// Follow symlinks to get actual file permissions
	for {
		fileInfo, err = os.Stat(filePath)
		if err != nil {
			t.Errorf("Failed to get file info for %s: %v", fileName, err)
			return
		}

		if fileInfo.Mode()&os.ModeSymlink != 0 {
			filePath, err = os.Readlink(filePath)
			if err != nil {
				t.Errorf("Failed to read symlink %s: %v", fileName, err)
				return
			}
		} else {
			break
		}
	}

	// Check ownership (Linux-specific)
	if sysStat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		if expectedUID > 0 && sysStat.Uid != uint32(expectedUID) {
			t.Errorf("File %q has incorrect UID. Expected: %d, Got: %d",
				filePath, expectedUID, sysStat.Uid)
		}
		if expectedGID > 0 && sysStat.Gid != uint32(expectedGID) {
			t.Errorf("File %q has incorrect GID. Expected: %d, Got: %d",
				filePath, expectedGID, sysStat.Gid)
		}
	} else {
		t.Logf("Cannot verify ownership on this platform for file %q", filePath)
	}
}

func printDstStructure(t *testing.T, dstDir string) {
	t.Logf("Directory structure of %q:", dstDir)
	err := filepath.Walk(dstDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Errorf("Error walking path %q: %v", path, err)
			return nil
		}
		relPath, _ := filepath.Rel(dstDir, path)
		uid, gid := -1, -1
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			uid = int(stat.Uid)
			gid = int(stat.Gid)
		}
		t.Logf("Entry: %s, Mode: %s, uid: %d, gid: %d", relPath, info.Mode().String(), uid, gid)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk data directory: %v", err)
	}
}
