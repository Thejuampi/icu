package icu

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	configDirEnvVar          = "ICU_CONFIG_DIR"
	testIsolatedConfigEnvVar = "ICU_TEST_ISOLATED_CONFIG"
	configAuditFilename      = "audit.jsonl"
	defaultConfigSaveAction  = "save_config"
)

type ConfigStore interface {
	ConfigPath() string
	AuditPath() string
	Load() (*Config, error)
	Save(cfg *Config, action string) error
}

type ConfigAuditEntry struct {
	Timestamp       string   `json:"timestamp"`
	Action          string   `json:"action"`
	ConfigPath      string   `json:"configPath"`
	AuditPath       string   `json:"auditPath"`
	ChangedFields   []string `json:"changedFields,omitempty"`
	SecretMutations []string `json:"secretMutations,omitempty"`
	TestMode        bool     `json:"testMode"`
}

type FileConfigStore struct {
	readFile    func(string) ([]byte, error)
	writeFile   func(string, []byte, os.FileMode) error
	appendFile  func(string, []byte, os.FileMode) error
	mkdirAll    func(string, os.FileMode) error
	userHomeDir func() (string, error)
	getenv      func(string) string
	now         func() time.Time
	args        func() []string
	mu          sync.Mutex
}

type MemoryConfigStore struct {
	mu      sync.Mutex
	config  Config
	audits  []ConfigAuditEntry
	path    string
	audPath string
}

var (
	activeConfigStoreMu sync.RWMutex
	activeConfigStore   ConfigStore = NewFileConfigStore()
)

func NewFileConfigStore() *FileConfigStore {
	return &FileConfigStore{
		readFile:    os.ReadFile,
		writeFile:   os.WriteFile,
		appendFile:  appendFile,
		mkdirAll:    os.MkdirAll,
		userHomeDir: os.UserHomeDir,
		getenv:      os.Getenv,
		now:         time.Now,
		args:        func() []string { return os.Args },
	}
}

func NewMemoryConfigStore() *MemoryConfigStore {
	return &MemoryConfigStore{
		path:    "memory://config.json",
		audPath: "memory://audit.jsonl",
	}
}

func SetConfigStoreForTesting(store ConfigStore) func() {
	if !runningUnderGoTest(os.Args) {
		panic("SetConfigStoreForTesting is only allowed during tests")
	}
	if store == nil {
		panic("SetConfigStoreForTesting requires a non-nil store")
	}

	activeConfigStoreMu.Lock()
	previous := activeConfigStore
	activeConfigStore = store
	activeConfigStoreMu.Unlock()

	return func() {
		activeConfigStoreMu.Lock()
		activeConfigStore = previous
		activeConfigStoreMu.Unlock()
	}
}

func ConfigPath() string {
	return currentConfigStore().ConfigPath()
}

func ConfigAuditPath() string {
	return currentConfigStore().AuditPath()
}

func LoadConfig() (*Config, error) {
	return currentConfigStore().Load()
}

func SaveConfig(cfg *Config) error {
	return SaveConfigWithAction(cfg, defaultConfigSaveAction)
}

func SaveConfigWithAction(cfg *Config, action string) error {
	if action == "" {
		action = defaultConfigSaveAction
	}

	return currentConfigStore().Save(cfg, action)
}

func (store *MemoryConfigStore) ConfigPath() string {
	return store.path
}

func (store *MemoryConfigStore) AuditPath() string {
	return store.audPath
}

func (store *MemoryConfigStore) Load() (*Config, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	cfg := store.config

	return &cfg, nil
}

func (store *MemoryConfigStore) Save(cfg *Config, action string) error {
	if cfg == nil {
		return errors.New("cannot save nil config")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	previous := store.config
	store.config = *cfg
	store.audits = append(store.audits, buildConfigAuditEntry(action, store.path, store.audPath, &previous, cfg, time.Now(), true))

	return nil
}

func (store *MemoryConfigStore) AuditEntries() []ConfigAuditEntry {
	store.mu.Lock()
	defer store.mu.Unlock()

	copied := make([]ConfigAuditEntry, len(store.audits))
	copy(copied, store.audits)

	return copied
}

func (store *FileConfigStore) ConfigPath() string {
	return filepath.Join(store.configDir(), "config.json")
}

func (store *FileConfigStore) AuditPath() string {
	return filepath.Join(store.configDir(), configAuditFilename)
}

func (store *FileConfigStore) Load() (*Config, error) {
	path := store.ConfigPath()
	data, err := store.readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			var cfg Config

			return &cfg, nil
		}

		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if len(data) == 0 {
		return &cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

func (store *FileConfigStore) Save(cfg *Config, action string) error {
	if cfg == nil {
		return errors.New("cannot save nil config")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	previous, err := store.loadWithoutLock()
	if err != nil {
		return err
	}
	dir := store.configDir()
	if err := store.mkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	var data []byte
	data, err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	configPath := filepath.Join(dir, "config.json")
	if err := store.writeFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	audit := buildConfigAuditEntry(action, configPath, filepath.Join(dir, configAuditFilename), previous, cfg, store.now(), store.runningUnderTest())
	if err := store.appendAudit(audit); err != nil {
		return fmt.Errorf("config saved but audit write failed: %w", err)
	}

	return nil
}

func (store *FileConfigStore) appendAudit(entry ConfigAuditEntry) error {
	encoded, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode audit entry: %w", err)
	}

	encoded = append(encoded, '\n')

	return store.appendFile(store.AuditPath(), encoded, 0o600)
}

func (store *FileConfigStore) loadWithoutLock() (*Config, error) {
	path := store.ConfigPath()
	data, err := store.readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			var cfg Config

			return &cfg, nil
		}

		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if len(data) == 0 {
		return &cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

func (store *FileConfigStore) configDir() string {
	store.assertTestIsolation()

	dir := store.getenv(configDirEnvVar)
	if dir != "" {
		if store.runningUnderTest() {
			return filepath.Join(dir, sanitizeTestScopeName(currentTestScopeName()))
		}

		return dir
	}

	home, err := store.userHomeDir()
	if err != nil {
		return ".icu"
	}

	return filepath.Join(home, ".icu")
}

func (store *FileConfigStore) assertTestIsolation() {
	if !store.runningUnderTest() {
		return
	}

	if store.getenv(configDirEnvVar) != "" && store.getenv(testIsolatedConfigEnvVar) == "1" {
		return
	}

	panic("unsafe config access during tests: set ICU_CONFIG_DIR and ICU_TEST_ISOLATED_CONFIG=1")
}

func (store *FileConfigStore) runningUnderTest() bool {
	return runningUnderGoTest(store.args())
}

func runningUnderGoTest(args []string) bool {
	for index := range args {
		if strings.HasPrefix(args[index], "-test.") {
			return true
		}
	}
	if len(args) == 0 {
		return false
	}

	return strings.Contains(filepath.Base(args[0]), ".test")
}

func currentTestScopeName() string {
	pcs := make([]uintptr, 32)
	count := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:count])

	for {
		frame, more := frames.Next()
		parts := strings.Split(frame.Function, ".")
		for index := len(parts) - 1; index >= 0; index-- {
			if strings.HasPrefix(parts[index], "Test") || strings.HasPrefix(parts[index], "Benchmark") || strings.HasPrefix(parts[index], "Fuzz") {
				return parts[index]
			}
		}
		if !more {
			break
		}
	}

	return "process"
}

func sanitizeTestScopeName(name string) string {
	if name == "" {
		return "process"
	}

	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)

			continue
		}

		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "process"
	}

	return builder.String()
}

func buildConfigAuditEntry(action, configPath, auditPath string, previous *Config, next *Config, now time.Time, testMode bool) ConfigAuditEntry {
	var changedFields []string
	var secretMutations []string

	if previous == nil {
		previous = &Config{}
	}
	if next == nil {
		next = &Config{}
	}

	appendChangedField(&changedFields, previous.AthleteID, next.AthleteID, "athleteId")
	appendChangedField(&changedFields, previous.Output, next.Output, "output")
	appendSecretMutation(&secretMutations, previous.APIKey, next.APIKey, "apiKey")
	appendSecretMutation(&secretMutations, previous.ZeppLoginToken, next.ZeppLoginToken, "zeppLoginToken")
	appendSecretMutation(&secretMutations, previous.ZeppAppToken, next.ZeppAppToken, "zeppAppToken")
	appendChangedField(&changedFields, previous.ZeppUserID, next.ZeppUserID, "zeppUserId")
	appendChangedField(&changedFields, previous.ZeppCountryCode, next.ZeppCountryCode, "zeppCountryCode")

	return ConfigAuditEntry{
		Timestamp:       now.UTC().Format(time.RFC3339),
		Action:          action,
		ConfigPath:      configPath,
		AuditPath:       auditPath,
		ChangedFields:   changedFields,
		SecretMutations: secretMutations,
		TestMode:        testMode,
	}
}

func appendChangedField(changedFields *[]string, previous string, next string, name string) {
	if previous == next {
		return
	}

	*changedFields = append(*changedFields, name)
}

func appendSecretMutation(mutations *[]string, previous string, next string, name string) {
	if previous == next {
		return
	}

	*mutations = append(*mutations, classifySecretMutation(previous, next, name))
}

func classifySecretMutation(previous string, next string, name string) string {
	if previous == "" && next != "" {
		return name + ":set"
	}
	if previous != "" && next == "" {
		return name + ":cleared"
	}

	return name + ":rotated"
}

func appendFile(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)

	return err
}

func currentConfigStore() ConfigStore {
	activeConfigStoreMu.RLock()
	defer activeConfigStoreMu.RUnlock()

	return activeConfigStore
}
