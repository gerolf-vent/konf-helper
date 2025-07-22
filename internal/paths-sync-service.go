package internal

import (
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type PathsSyncService struct {
	pathConfigs []*PathConfig
	fsWatcher   *fsnotify.Watcher
	patchEngine *PatchEngine
	notifier    Notifier
	debouncer   *Debouncer

	isStarted  bool
	stop       chan struct{}
	healthPing chan uint64
	healthPong chan uint64
	mu         sync.Mutex

	logger *zap.Logger
}

func NewPathsSyncService(debounceDelay time.Duration) (*PathsSyncService, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &PathsSyncService{
		fsWatcher:   fsWatcher,
		patchEngine: NewPatchEngine(),
		debouncer:   NewDebouncer(debounceDelay),

		logger: zap.L(),
	}, nil
}

func (pss *PathsSyncService) SetPathConfig(pathConfig *PathConfig) error {
	pss.mu.Lock()
	defer pss.mu.Unlock()

	srcPath := pathConfig.SrcPath()

	// Watch the path for changes
	if err := pss.fsWatcher.Add(srcPath); err != nil {
		return fmt.Errorf("failed to add path to fs watcher: %w", err)
	}

	// Allow templates to read files from the path
	pss.patchEngine.AllowPath(srcPath)

	pss.pathConfigs = append(pss.pathConfigs, pathConfig)
	pss.logger.Debug("added path config", zap.Object("path-config", pathConfig))
	return nil
}

func (pss *PathsSyncService) SetNotifier(notifier Notifier) {
	pss.mu.Lock()
	defer pss.mu.Unlock()
	pss.notifier = notifier
	pss.logger.Debug("set notifier", zap.Object("notifier", notifier))
}

func (pss *PathsSyncService) Start() error {
	pss.mu.Lock()
	defer pss.mu.Unlock()

	if pss.isStarted {
		return nil // Already started
	}

	pss.logger.Debug("starting paths sync service")

	pss.stop = make(chan struct{})
	pss.healthPing = make(chan uint64, 5)
	pss.healthPong = make(chan uint64, 5)

	go pss.watch()

	pss.logger.Debug("performing initial sync of paths")
	syncOk := pss.sync()
	if !syncOk {
		pss.logger.Error("failed to sync paths")
	} else {
		pss.logger.Debug("successfully synced paths")
	}

	pss.isStarted = true
	pss.logger.Info("paths sync service started")
	return nil
}

func (pss *PathsSyncService) Stop() {
	pss.mu.Lock()
	defer pss.mu.Unlock()

	if !pss.isStarted {
		return
	}

	pss.logger.Debug("stopping paths sync service")

	close(pss.stop)
	close(pss.healthPing)
	close(pss.healthPong)
	pss.isStarted = false

	pss.logger.Info("paths sync service stopped")
}

func (pss *PathsSyncService) IsStarted() bool {
	pss.mu.Lock()
	defer pss.mu.Unlock()
	return pss.isStarted
}

func (pss *PathsSyncService) CheckHealth() error {
	pss.mu.Lock()
	defer pss.mu.Unlock()

	if !pss.isStarted {
		return fmt.Errorf("paths sync service is not started")
	}

	pingID := rand.Uint64()
	timeout := time.After(5 * time.Second)

	select {
	case pss.healthPing <- pingID:
		for {
			select {
			case pongID, ok := <-pss.healthPong:
				if !ok {
					return fmt.Errorf("health ping channel closed")
				}
				if pongID == pingID {
					return nil // Health check successful
				}
			case <-timeout:
				return fmt.Errorf("health check timed out")
			}
		}
	default:
		return fmt.Errorf("failed to send health ping")
	}
}

func (pss *PathsSyncService) watch() {
	for {
		select {
		case id, ok := <-pss.healthPing:
			if !ok {
				pss.logger.Debug("health ping channel closed")
				return
			}
			pss.healthPong <- id // Respond to health ping
			pss.logger.Debug("health ping bounced", zap.Uint64("id", id))
		case <-pss.stop:
			pss.logger.Info("fs watcher stopped")
			return
		case event, ok := <-pss.fsWatcher.Events:
			if !ok {
				pss.logger.Debug("fs watcher events channel closed")
				return
			}
			pss.mu.Lock()

			fileName := path.Base(event.Name)
			if strings.HasPrefix(fileName, ".") && fileName != "..data" {
				// Ignore hidden files and directories except for "..data"
				pss.mu.Unlock()
				continue
			}

			pss.logger.Debug("received fs event",
				zap.String("path", event.Name),
				zap.String("event", event.Op.String()),
			)

			pss.debouncer.Call(func() {
				pss.mu.Lock()
				defer pss.mu.Unlock()

				syncOK := pss.sync()
				if !syncOK {
					pss.logger.Error("failed to sync paths")
				} else {
					pss.logger.Info("successfully synced paths")
				}

				if pss.notifier != nil {
					notifyOK := pss.notifier.Notify()
					if !notifyOK {
						pss.logger.Error("failed to notify process",
							zap.Object("notifier", pss.notifier),
						)
					} else {
						pss.logger.Info("successfully notified process",
							zap.Object("notifier", pss.notifier),
						)
					}
				}
			})
			pss.mu.Unlock()
		case err, ok := <-pss.fsWatcher.Errors:
			if !ok {
				pss.logger.Debug("fs watcher errors channel closed")
				return
			}
			pss.logger.Error("fs watcher error", zap.Error(err))
		}
	}
}

func (pss *PathsSyncService) sync() bool {
	// !! Locking must be handled by the caller !!
	ok := true

	// Create a map to group path configurations by destination paths
	dstPaths := make(map[string][]*PathConfig)
	for _, pc := range pss.pathConfigs {
		if !pc.HasDstPath() {
			continue // Skip paths without a destination
		}
		dstPath := pc.DstPath()
		dstPaths[dstPath] = append(dstPaths[dstPath], pc)
	}

	dstDataDirName := time.Now().Format("..2006_01_02_15_04_05.000000000")

	for dstPath, pcs := range dstPaths {
		// Get all existing file names in the destination directory
		dstDirEntries, err := os.ReadDir(dstPath)
		if err != nil {
			pss.logger.Error("failed to read destination directory",
				zap.String("path", dstPath),
				zap.Error(err),
			)
			ok = false
			continue
		}

		// Existing files in the destination directory
		existingDstFileNames := make(map[string]struct{})
		// Existing data directories in the destination directory
		var existingDstDataDirNames []string
		for _, entry := range dstDirEntries {
			fileName := entry.Name()
			if fileName[:2] == ".." && entry.Type()&os.ModeDir != 0 {
				existingDstDataDirNames = append(existingDstDataDirNames, fileName)
				continue
			}
			if fileName == "" || fileName[0] == '.' || entry.IsDir() {
				continue
			}
			existingDstFileNames[fileName] = struct{}{}
		}
		pss.logger.Debug("sucessfully indexed existing files in destination directory",
			zap.String("dstPath", dstPath),
			zap.Int("count", len(existingDstFileNames)),
			zap.Strings("fileNames", slices.Sorted(maps.Keys(existingDstFileNames))),
		)
		pss.logger.Debug("sucessfully indexed existing data directories in destination directory",
			zap.String("dstPath", dstPath),
			zap.Int("count", len(existingDstDataDirNames)),
			zap.Strings("dirNames", existingDstDataDirNames),
		)

		dstDataPath := filepath.Join(dstPath, dstDataDirName)

		if err := os.Mkdir(dstDataPath, 0755); err != nil {
			pss.logger.Error("failed to create destination data directory",
				zap.String("path", dstDataPath),
				zap.Error(err),
			)
			ok = false
			continue
		} else {
			pss.logger.Debug("successfully created destination data directory",
				zap.String("path", dstDataPath),
			)
		}

		dstDataMergedPath := filepath.Join(dstDataPath, "merged")

		if err := os.Mkdir(dstDataMergedPath, 0755); err != nil {
			pss.logger.Error("failed to create destination data directory",
				zap.String("path", dstDataPath),
				zap.String("subPath", "merged"),
				zap.Error(err),
			)
			ok = false
			continue
		} else {
			pss.logger.Debug("successfully created destination data directory",
				zap.String("path", dstDataPath),
				zap.String("subPath", "merged"),
			)
		}

		linkMap := make(map[string]string)

		for _, pc := range pcs {
			srcPath := pc.SrcPath()
			pcID, err := pc.ID()
			if err != nil {
				pss.logger.Error("failed to generate path config id",
					zap.String("srcPath", srcPath),
					zap.Error(err),
				)
				ok = false
				continue
			}

			pcDstDataPath := filepath.Join(dstDataPath, pcID)

			// Create destination subdirectory for the specific path config
			if err := os.Mkdir(pcDstDataPath, 0700); err != nil {
				pss.logger.Error("failed to create destination data directory",
					zap.String("path", dstDataPath),
					zap.String("subPath", pcID),
					zap.Error(err),
				)
				ok = false
				continue
			} else {
				pss.logger.Debug("successfully created destination data directory",
					zap.String("path", dstDataPath),
					zap.String("subPath", pcID),
				)
			}

			// Get all files from the source directory
			srcDirEntries, err := os.ReadDir(srcPath)
			if err != nil {
				pss.logger.Error("failed to read source directory",
					zap.String("path", srcPath),
					zap.Error(err),
				)
				ok = false
				continue
			}

			// Get all relevant file names from the source directory
			var srcFileNames []string
			for _, entry := range srcDirEntries {
				fileName := entry.Name()
				if fileName == "" || fileName[0] == '.' {
					continue
				}
				if entry.IsDir() {
					continue
				}
				if pc.HasFileGlob() {
					matched, err := filepath.Match(pc.FileGlob(), fileName)
					if err != nil {
						pss.logger.Error("failed to match file glob",
							zap.String("glob", pc.FileGlob()),
							zap.String("file", fileName),
							zap.Error(err),
						)
						ok = false
						continue
					}
					if !matched {
						continue
					}
				}
				srcFileNames = append(srcFileNames, fileName)
			}
			pss.logger.Debug("successfully indexed source files",
				zap.String("srcPath", srcPath),
				zap.Int("count", len(srcFileNames)),
				zap.Strings("fileNames", srcFileNames),
			)

			// Prepare file and directory modes
			var dirMode os.FileMode = 0755
			var fileMode os.FileMode = 0640
			if pc.HasMode() {
				dirMode = os.FileMode(pc.Mode()) | 0511
				fileMode = os.FileMode(pc.Mode()) | 0400
			}

			// Set owner and group if specified
			uid := -1
			if pc.HasOwner() {
				uid = int(pc.Owner())
			}
			gid := -1
			if pc.HasGroup() {
				gid = int(pc.Group())
			}

			// Copy files from the source directory to the destination directory
			for _, fileName := range srcFileNames {
				srcFile := filepath.Join(srcPath, fileName)
				dstFile := filepath.Join(pcDstDataPath, fileName)

				srcReader, err := os.Open(srcFile)
				if err != nil {
					pss.logger.Error("failed to open source file",
						zap.String("path", srcFile),
						zap.Error(err),
					)
					ok = false
					continue
				}
				dstWriter, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
				if err != nil {
					srcReader.Close()
					pss.logger.Error("failed to open destination file",
						zap.String("path", dstFile),
						zap.Error(err),
					)
					ok = false
					continue
				}

				if pss.patchEngine != nil {
					err = pss.patchEngine.Patch(srcReader, dstWriter)
				} else {
					_, err = io.Copy(dstWriter, srcReader)
				}

				srcReader.Close()
				dstWriter.Close()
				if err != nil {
					pss.logger.Error("failed to patch/copy file",
						zap.String("srcPath", srcFile),
						zap.String("dstPath", dstFile),
						zap.Error(err),
					)
					ok = false
					continue
				} else {
					pss.logger.Debug("successfully patched/copied file",
						zap.String("srcPath", srcFile),
						zap.String("dstPath", dstFile),
					)
				}

				if err = os.Chmod(dstFile, fileMode); err != nil {
					pss.logger.Error("failed to set file mode",
						zap.String("path", dstFile),
						zap.String("mode", fileMode.String()),
						zap.Error(err),
					)
					ok = false
					continue
				} else {
					pss.logger.Debug("successfully set file mode",
						zap.String("path", dstFile),
						zap.String("mode", fileMode.String()),
					)
				}

				if uid > 0 || gid > 0 {
					if err = os.Chown(dstFile, uid, gid); err != nil {
						pss.logger.Error("failed to set file owner/group",
							zap.String("path", dstFile),
							zap.Int("uid", uid),
							zap.Int("gid", gid),
							zap.Error(err),
						)
						ok = false
						continue
					} else {
						pss.logger.Debug("successfully set file owner/group",
							zap.String("path", dstFile),
							zap.Int("uid", uid),
							zap.Int("gid", gid),
						)
					}
				}

				linkMap[fileName] = pcID
			}

			if err = os.Chmod(pcDstDataPath, dirMode); err != nil {
				pss.logger.Error("failed to set directory mode",
					zap.String("path", pcDstDataPath),
					zap.String("mode", dirMode.String()),
					zap.Error(err),
				)
				ok = false
				continue
			} else {
				pss.logger.Debug("successfully set directory mode",
					zap.String("path", pcDstDataPath),
					zap.String("mode", dirMode.String()),
				)
			}

			if uid > 0 || gid > 0 {
				if err = os.Chown(pcDstDataPath, uid, gid); err != nil {
					pss.logger.Error("failed to set directory owner/group",
						zap.String("path", pcDstDataPath),
						zap.Int("uid", uid),
						zap.Int("gid", gid),
						zap.Error(err),
					)
					ok = false
					continue
				} else {
					pss.logger.Debug("successfully set directory owner/group",
						zap.String("path", pcDstDataPath),
						zap.Int("uid", uid),
						zap.Int("gid", gid),
					)
				}
			}
		}

		// Create symlinks in the data directory ("..$timestamp/merged/$file" -> "../$pcID/$file")
		for fileName, pcID := range linkMap {
			linkPath := filepath.Join(dstDataMergedPath, fileName)
			linkTarget := filepath.Join("..", pcID, fileName)

			if err := os.Symlink(linkTarget, linkPath); err != nil {
				pss.logger.Error("failed to create symlink",
					zap.String("path", linkPath),
					zap.String("target", linkTarget),
					zap.Error(err),
				)
				ok = false
				continue
			} else {
				pss.logger.Debug("successfully created symlink",
					zap.String("path", linkPath),
					zap.String("target", linkTarget),
				)
			}

			if _, exists := existingDstFileNames[fileName]; !exists {
				linkPath := filepath.Join(dstPath, fileName)
				linkTarget := filepath.Join("..data", fileName)
				if err := os.Symlink(linkTarget, linkPath); err != nil {
					pss.logger.Error("failed to create symlink",
						zap.String("path", linkPath),
						zap.String("target", linkTarget),
						zap.Error(err),
					)
					ok = false
					continue
				} else {
					pss.logger.Debug("successfully created symlink",
						zap.String("path", linkPath),
						zap.String("target", linkTarget),
					)
				}
			}
		}

		// Update the "..data" symlink to point to the new data directory
		dataLinkPath := filepath.Join(dstPath, "..data")
		if err := os.Remove(dataLinkPath); err != nil && !os.IsNotExist(err) {
			pss.logger.Error("failed to remove old data link",
				zap.String("path", dataLinkPath),
				zap.Error(err),
			)
			ok = false
			continue
		} else {
			pss.logger.Debug("successfully removed old data link",
				zap.String("path", dataLinkPath),
			)
		}
		if err := os.Symlink(filepath.Join(dstDataDirName, "merged"), dataLinkPath); err != nil {
			pss.logger.Error("failed to create new data link",
				zap.String("path", dataLinkPath),
				zap.String("target", dstDataDirName),
				zap.Error(err),
			)
			ok = false
			continue
		} else {
			pss.logger.Debug("successfully created new data link",
				zap.String("path", dataLinkPath),
				zap.String("target", dstDataDirName),
			)
		}

		// Remove old files in the destination directory
		for fileName := range existingDstFileNames {
			if _, exists := linkMap[fileName]; !exists {
				filePath := filepath.Join(dstPath, fileName)
				if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
					pss.logger.Error("failed to remove old file",
						zap.String("path", filePath),
						zap.Error(err),
					)
					ok = false
					continue
				} else {
					pss.logger.Debug("successfully removed old file",
						zap.String("path", filePath),
					)
				}
			}
		}

		// Remove old data directories in the destination directory
		for _, dirName := range existingDstDataDirNames {
			dirPath := filepath.Join(dstPath, dirName)
			if err := os.RemoveAll(dirPath); err != nil && !os.IsNotExist(err) {
				pss.logger.Error("failed to remove old data directory",
					zap.String("path", dirPath),
					zap.Error(err),
				)
				ok = false
				continue
			} else {
				pss.logger.Debug("successfully removed old data directory",
					zap.String("path", dirPath),
				)
			}
		}
	}

	return ok
}
