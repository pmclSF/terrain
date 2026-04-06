package convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pmclSF/terrain/internal/analysis"
)

const (
	migrationStateVersion  = 1
	migrationStateDirName  = "migration"
	migrationStateFilename = "state.json"
)

type MigrationFileRecord struct {
	InputPath   string   `json:"inputPath"`
	OutputPath  string   `json:"outputPath,omitempty"`
	Type        string   `json:"type"`
	Framework   string   `json:"framework,omitempty"`
	Status      string   `json:"status"`
	Confidence  int      `json:"confidence"`
	TodosAdded  int      `json:"todosAdded,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Error       string   `json:"error,omitempty"`
	SkipReason  string   `json:"skipReason,omitempty"`
	ConvertedAt string   `json:"convertedAt,omitempty"`
}

type MigrationEstimateSummary struct {
	TotalFiles      int `json:"totalFiles"`
	TestFiles       int `json:"testFiles"`
	HelperFiles     int `json:"helperFiles"`
	ConfigFiles     int `json:"configFiles"`
	OtherFiles      int `json:"otherFiles"`
	PredictedHigh   int `json:"predictedHigh"`
	PredictedMedium int `json:"predictedMedium"`
	PredictedLow    int `json:"predictedLow"`
}

type MigrationEffortEstimate struct {
	LowConfidenceFiles    int    `json:"lowConfidenceFiles"`
	MediumConfidenceFiles int    `json:"mediumConfidenceFiles"`
	EstimatedManualMins   int    `json:"estimatedManualMinutes"`
	Description           string `json:"description"`
}

type MigrationBlocker struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
	Impact  string `json:"impact"`
}

type MigrationEstimate struct {
	Root            string                   `json:"root"`
	From            string                   `json:"from"`
	To              string                   `json:"to"`
	Summary         MigrationEstimateSummary `json:"summary"`
	Files           []MigrationFileRecord    `json:"files"`
	Blockers        []MigrationBlocker       `json:"blockers"`
	EstimatedEffort MigrationEffortEstimate  `json:"estimatedEffort"`
}

type MigrationStatus struct {
	Total      int    `json:"total"`
	Converted  int    `json:"converted"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
	Pending    int    `json:"pending"`
	Source     string `json:"source,omitempty"`
	Target     string `json:"target,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	OutputRoot string `json:"outputRoot,omitempty"`
}

type MigrationRunOptions struct {
	Output         string `json:"output,omitempty"`
	Continue       bool   `json:"continue,omitempty"`
	RetryFailed    bool   `json:"retryFailed,omitempty"`
	StrictValidate bool   `json:"strictValidate,omitempty"`
	Concurrency    int    `json:"concurrency,omitempty"`
}

type MigrationResult struct {
	Root      string                `json:"root"`
	From      string                `json:"from"`
	To        string                `json:"to"`
	Output    string                `json:"output,omitempty"`
	Processed []MigrationFileRecord `json:"processed"`
	Checklist string                `json:"checklist,omitempty"`
	State     MigrationStatus       `json:"state"`
}

type MigrationDoctorCheck struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	Verbose     string `json:"verbose,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

type MigrationDoctorSummary struct {
	Pass  int `json:"pass"`
	Warn  int `json:"warn"`
	Fail  int `json:"fail"`
	Total int `json:"total"`
}

type MigrationDoctorResult struct {
	Checks  []MigrationDoctorCheck `json:"checks"`
	Summary MigrationDoctorSummary `json:"summary"`
	HasFail bool                   `json:"hasFail"`
}

type migrationCandidate struct {
	AbsPath    string
	RelPath    string
	Type       string
	Framework  string
	Language   string
	IsConfig   bool
	ConfigName string
}

type migrationStateFile struct {
	Status      string `json:"status"`
	ConvertedAt string `json:"convertedAt,omitempty"`
	Confidence  int    `json:"confidence,omitempty"`
	Error       string `json:"error,omitempty"`
	Reason      string `json:"reason,omitempty"`
	OutputPath  string `json:"outputPath,omitempty"`
	FileType    string `json:"fileType,omitempty"`
	TodosAdded  int    `json:"todosAdded,omitempty"`
}

type migrationState struct {
	Version    int                           `json:"version"`
	StartedAt  string                        `json:"startedAt"`
	UpdatedAt  string                        `json:"updatedAt,omitempty"`
	Source     string                        `json:"source,omitempty"`
	Target     string                        `json:"target,omitempty"`
	OutputRoot string                        `json:"outputRoot,omitempty"`
	Files      map[string]migrationStateFile `json:"files"`
}

type MigrationStateManager struct {
	projectRoot     string
	stateDir        string
	statePath       string
	legacyStatePath string
	state           *migrationState
}

func NewMigrationStateManager(projectRoot string) *MigrationStateManager {
	root := filepath.Clean(projectRoot)
	stateDir := filepath.Join(root, ".terrain", migrationStateDirName)
	return &MigrationStateManager{
		projectRoot:     root,
		stateDir:        stateDir,
		statePath:       filepath.Join(stateDir, migrationStateFilename),
		legacyStatePath: filepath.Join(root, ".terrain", "state.json"),
	}
}

func (m *MigrationStateManager) Init(source, target, outputRoot string) error {
	now := nowRFC3339()
	m.state = &migrationState{
		Version:    migrationStateVersion,
		StartedAt:  now,
		UpdatedAt:  now,
		Source:     source,
		Target:     target,
		OutputRoot: outputRoot,
		Files:      map[string]migrationStateFile{},
	}
	return m.Save()
}

func (m *MigrationStateManager) Exists() (bool, error) {
	if _, err := os.Stat(m.statePath); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if _, err := os.Stat(m.legacyStatePath); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}

func (m *MigrationStateManager) Load() error {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			data, err = os.ReadFile(m.legacyStatePath)
		}
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("no migration state found; run `terrain migrate` to start")
			}
			return fmt.Errorf("read migration state: %w", err)
		}
	}

	var state migrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode migration state: %w", err)
	}
	if state.Files == nil {
		state.Files = map[string]migrationStateFile{}
	}
	m.state = &state
	return nil
}

func (m *MigrationStateManager) Save() error {
	if m.state == nil {
		return fmt.Errorf("no migration state loaded")
	}
	m.state.UpdatedAt = nowRFC3339()
	if err := os.MkdirAll(m.stateDir, 0o755); err != nil {
		return fmt.Errorf("create migration state dir: %w", err)
	}
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode migration state: %w", err)
	}
	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write migration state: %w", err)
	}
	if err := os.Rename(tmp, m.statePath); err != nil {
		return fmt.Errorf("persist migration state: %w", err)
	}
	return nil
}

func (m *MigrationStateManager) Reset() error {
	if err := os.RemoveAll(m.stateDir); err != nil {
		return fmt.Errorf("remove migration state dir: %w", err)
	}
	if err := os.Remove(m.legacyStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove legacy migration state: %w", err)
	}
	m.state = nil
	return nil
}

func (m *MigrationStateManager) State() *migrationState {
	return m.state
}

func (m *MigrationStateManager) MarkConverted(relPath, outputPath, fileType string, confidence, todos int) {
	if m.state == nil {
		return
	}
	m.state.Files[filepath.Clean(relPath)] = migrationStateFile{
		Status:      "converted",
		ConvertedAt: nowRFC3339(),
		Confidence:  confidence,
		OutputPath:  outputPath,
		FileType:    fileType,
		TodosAdded:  todos,
	}
}

func (m *MigrationStateManager) MarkFailed(relPath, fileType, outputPath string, err error) {
	if m.state == nil {
		return
	}
	m.state.Files[filepath.Clean(relPath)] = migrationStateFile{
		Status:      "failed",
		ConvertedAt: nowRFC3339(),
		Confidence:  0,
		OutputPath:  outputPath,
		FileType:    fileType,
		Error:       strings.TrimSpace(err.Error()),
	}
}

func (m *MigrationStateManager) MarkSkipped(relPath, fileType, reason string) {
	if m.state == nil {
		return
	}
	m.state.Files[filepath.Clean(relPath)] = migrationStateFile{
		Status:      "skipped",
		ConvertedAt: nowRFC3339(),
		FileType:    fileType,
		Reason:      strings.TrimSpace(reason),
	}
}

func (m *MigrationStateManager) IsConverted(relPath string) bool {
	if m.state == nil {
		return false
	}
	entry, ok := m.state.Files[filepath.Clean(relPath)]
	return ok && entry.Status == "converted"
}

func (m *MigrationStateManager) IsFailed(relPath string) bool {
	if m.state == nil {
		return false
	}
	entry, ok := m.state.Files[filepath.Clean(relPath)]
	return ok && entry.Status == "failed"
}

func (m *MigrationStateManager) Status() MigrationStatus {
	if m.state == nil {
		return MigrationStatus{}
	}
	status := MigrationStatus{
		Source:     m.state.Source,
		Target:     m.state.Target,
		StartedAt:  m.state.StartedAt,
		UpdatedAt:  m.state.UpdatedAt,
		OutputRoot: m.state.OutputRoot,
	}
	for _, entry := range m.state.Files {
		status.Total++
		switch entry.Status {
		case "converted":
			status.Converted++
		case "failed":
			status.Failed++
		case "skipped":
			status.Skipped++
		default:
			status.Pending++
		}
	}
	return status
}

func (m *MigrationStateManager) Records() []MigrationFileRecord {
	if m.state == nil {
		return nil
	}
	paths := make([]string, 0, len(m.state.Files))
	for path := range m.state.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	records := make([]MigrationFileRecord, 0, len(paths))
	for _, path := range paths {
		entry := m.state.Files[path]
		record := MigrationFileRecord{
			InputPath:   path,
			OutputPath:  entry.OutputPath,
			Type:        entry.FileType,
			Status:      entry.Status,
			Confidence:  entry.Confidence,
			TodosAdded:  entry.TodosAdded,
			Error:       entry.Error,
			SkipReason:  entry.Reason,
			ConvertedAt: entry.ConvertedAt,
		}
		records = append(records, record)
	}
	return records
}

func EstimateMigration(root, from, to string) (MigrationEstimate, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return MigrationEstimate{}, fmt.Errorf("root is required")
	}
	from = NormalizeFramework(from)
	to = NormalizeFramework(to)
	direction, ok := LookupDirection(from, to)
	if !ok {
		return MigrationEstimate{}, fmt.Errorf("unsupported conversion: %s -> %s", from, to)
	}

	candidates, err := discoverMigrationCandidates(root, from)
	if err != nil {
		return MigrationEstimate{}, err
	}

	estimate := MigrationEstimate{
		Root:  root,
		From:  from,
		To:    to,
		Files: make([]MigrationFileRecord, 0, len(candidates)),
	}
	blockerCounts := map[string]int{}
	for _, candidate := range candidates {
		record, blockers := estimateCandidate(candidate, direction)
		estimate.Files = append(estimate.Files, record)
		accumulateEstimateSummary(&estimate.Summary, record)
		for _, blocker := range blockers {
			blockerCounts[blocker]++
		}
	}
	estimate.Blockers = blockersFromCounts(blockerCounts)
	estimate.EstimatedEffort = estimateEffort(estimate.Files)
	return estimate, nil
}

func MigrateProject(root, from, to string, options MigrationRunOptions) (MigrationResult, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return MigrationResult{}, fmt.Errorf("root is required")
	}
	from = NormalizeFramework(from)
	to = NormalizeFramework(to)
	direction, ok := LookupDirection(from, to)
	if !ok {
		return MigrationResult{}, fmt.Errorf("unsupported conversion: %s -> %s", from, to)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("resolve root: %w", err)
	}
	state := NewMigrationStateManager(rootAbs)
	outputRoot, err := prepareMigrationState(state, from, to, rootAbs, options)
	if err != nil {
		return MigrationResult{}, err
	}

	candidates, err := discoverMigrationCandidates(rootAbs, from)
	if err != nil {
		return MigrationResult{}, err
	}

	selected := make([]migrationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if options.Continue && !options.RetryFailed && state.IsConverted(candidate.RelPath) {
			continue
		}
		if options.RetryFailed && !state.IsFailed(candidate.RelPath) {
			continue
		}
		selected = append(selected, candidate)
	}

	outcomes := processMigrationCandidates(selected, direction, outputRoot, true, options.Concurrency)
	processed := make([]MigrationFileRecord, 0, len(outcomes))
	for _, outcome := range outcomes {
		candidate := outcome.candidate
		record := outcome.record
		err := outcome.err
		if err != nil {
			state.MarkFailed(candidate.RelPath, candidate.Type, record.OutputPath, err)
			if saveErr := state.Save(); saveErr != nil {
				return MigrationResult{}, saveErr
			}
			record.Status = "failed"
			record.Error = strings.TrimSpace(err.Error())
			processed = append(processed, record)
			continue
		}

		switch record.Status {
		case "converted":
			state.MarkConverted(candidate.RelPath, record.OutputPath, candidate.Type, record.Confidence, record.TodosAdded)
		case "skipped":
			state.MarkSkipped(candidate.RelPath, candidate.Type, record.SkipReason)
		default:
			state.MarkFailed(candidate.RelPath, candidate.Type, record.OutputPath, fmt.Errorf(record.Error))
		}
		if saveErr := state.Save(); saveErr != nil {
			return MigrationResult{}, saveErr
		}
		processed = append(processed, record)
	}

	checklist := GenerateMigrationChecklist(state.Records())
	return MigrationResult{
		Root:      rootAbs,
		From:      from,
		To:        to,
		Output:    outputRoot,
		Processed: processed,
		Checklist: checklist,
		State:     state.Status(),
	}, nil
}

type migrationOutcome struct {
	index     int
	candidate migrationCandidate
	record    MigrationFileRecord
	err       error
}

func processMigrationCandidates(candidates []migrationCandidate, direction Direction, outputRoot string, strictValidate bool, concurrency int) []migrationOutcome {
	if len(candidates) == 0 {
		return nil
	}

	type candidateJob struct {
		index     int
		candidate migrationCandidate
	}

	workerCount := clampWorkerCount(concurrency, len(candidates))
	jobs := make(chan candidateJob, len(candidates))
	results := make(chan migrationOutcome, len(candidates))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				record, err := migrateCandidate(job.candidate, direction, outputRoot, strictValidate)
				results <- migrationOutcome{
					index:     job.index,
					candidate: job.candidate,
					record:    record,
					err:       err,
				}
			}
		}()
	}

	for index, candidate := range candidates {
		jobs <- candidateJob{index: index, candidate: candidate}
	}
	close(jobs)
	wg.Wait()
	close(results)

	ordered := make([]migrationOutcome, len(candidates))
	for outcome := range results {
		ordered[outcome.index] = outcome
	}
	return ordered
}

func LoadMigrationStatus(root string) (MigrationStatus, bool, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return MigrationStatus{}, false, fmt.Errorf("resolve root: %w", err)
	}
	state := NewMigrationStateManager(rootAbs)
	exists, err := state.Exists()
	if err != nil {
		return MigrationStatus{}, false, err
	}
	if !exists {
		return MigrationStatus{}, false, nil
	}
	if err := state.Load(); err != nil {
		return MigrationStatus{}, true, err
	}
	return state.Status(), true, nil
}

func GenerateChecklistFromState(root string) (string, bool, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", false, fmt.Errorf("resolve root: %w", err)
	}
	state := NewMigrationStateManager(rootAbs)
	exists, err := state.Exists()
	if err != nil {
		return "", false, err
	}
	if !exists {
		return "", false, nil
	}
	if err := state.Load(); err != nil {
		return "", true, err
	}
	return GenerateMigrationChecklist(state.Records()), true, nil
}

func ResetMigrationState(root string) (bool, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return false, fmt.Errorf("resolve root: %w", err)
	}
	state := NewMigrationStateManager(rootAbs)
	exists, err := state.Exists()
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	return true, state.Reset()
}

func RunMigrationDoctor(root string) (MigrationDoctorResult, error) {
	rootAbs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return MigrationDoctorResult{}, fmt.Errorf("resolve root: %w", err)
	}
	checks := make([]MigrationDoctorCheck, 0, 6)

	pathCheck := doctorPathCheck(rootAbs)
	checks = append(checks, pathCheck)
	if pathCheck.Status == "FAIL" {
		return summarizeDoctorChecks(checks), nil
	}

	checks = append(checks, doctorWritableCheck(rootAbs))
	checks = append(checks, doctorProjectConfigCheck(rootAbs))
	checks = append(checks, doctorTestInventoryCheck(rootAbs))
	checks = append(checks, doctorSupportedDirectionsCheck(rootAbs))
	checks = append(checks, doctorMigrationStateCheck(rootAbs))
	return summarizeDoctorChecks(checks), nil
}

func GenerateMigrationChecklist(records []MigrationFileRecord) string {
	lines := []string{
		"# Migration Checklist",
		"",
		fmt.Sprintf("- **Total files:** %d", len(records)),
		fmt.Sprintf("- **High confidence (>=90%%):** %d", countRecordsByConfidence(records, 90, 101)),
		fmt.Sprintf("- **Medium confidence (70-89%%):** %d", countRecordsByConfidence(records, 70, 90)),
		fmt.Sprintf("- **Low confidence (<70%%):** %d", countRecordsByConfidence(records, 1, 70)),
		fmt.Sprintf("- **Failed/Manual:** %d", countManualRecords(records)),
	}

	appendChecklistSection := func(title string, filter func(MigrationFileRecord) bool, checked bool) {
		filtered := make([]MigrationFileRecord, 0, len(records))
		for _, record := range records {
			if filter(record) {
				filtered = append(filtered, record)
			}
		}
		if len(filtered) == 0 {
			return
		}
		lines = append(lines, "", "## "+title, "")
		box := "[ ]"
		if checked {
			box = "[x]"
		}
		for _, record := range filtered {
			lines = append(lines, fmt.Sprintf("- %s `%s` (%d%%)", box, record.InputPath, record.Confidence))
			if record.SkipReason != "" {
				lines = append(lines, "  - Reason: "+record.SkipReason)
			}
			if record.Error != "" {
				lines = append(lines, "  - Error: "+record.Error)
			}
			if record.TodosAdded > 0 {
				lines = append(lines, fmt.Sprintf("  - TODO comments inserted: %d", record.TodosAdded))
			}
		}
	}

	appendChecklistSection("Fully Converted", func(record MigrationFileRecord) bool {
		return record.Status == "converted" && record.Confidence >= 90
	}, true)
	appendChecklistSection("Needs Review", func(record MigrationFileRecord) bool {
		return record.Status == "converted" && record.Confidence >= 1 && record.Confidence < 90
	}, false)
	appendChecklistSection("Manual Steps Required", func(record MigrationFileRecord) bool {
		return record.Status == "failed" || record.Status == "skipped" || record.Confidence == 0
	}, false)
	appendChecklistSection("Config Changes", func(record MigrationFileRecord) bool {
		return record.Type == "config"
	}, false)

	return ensureTrailingNewline(strings.Join(lines, "\n"))
}

func prepareMigrationState(state *MigrationStateManager, from, to, root string, options MigrationRunOptions) (string, error) {
	outputRoot := strings.TrimSpace(options.Output)
	if outputRoot == "" {
		outputRoot = root
	} else {
		absOutput, err := filepath.Abs(outputRoot)
		if err != nil {
			return "", fmt.Errorf("resolve output: %w", err)
		}
		outputRoot = absOutput
	}

	exists, err := state.Exists()
	if err != nil {
		return "", err
	}
	if options.Continue || options.RetryFailed {
		if exists {
			if err := state.Load(); err != nil {
				return "", err
			}
			if state.state.Source != "" && state.state.Source != from {
				return "", fmt.Errorf("existing migration state targets %s -> %s, not %s -> %s", state.state.Source, state.state.Target, from, to)
			}
			if state.state.Target != "" && state.state.Target != to {
				return "", fmt.Errorf("existing migration state targets %s -> %s, not %s -> %s", state.state.Source, state.state.Target, from, to)
			}
			if state.state.OutputRoot != "" && strings.TrimSpace(options.Output) == "" {
				outputRoot = state.state.OutputRoot
			}
			if state.state.OutputRoot != "" && outputRoot != state.state.OutputRoot {
				return "", fmt.Errorf("existing migration state uses output %s; pass the same --output or reset the state", state.state.OutputRoot)
			}
			return outputRoot, nil
		}
		if err := state.Init(from, to, outputRoot); err != nil {
			return "", err
		}
		return outputRoot, nil
	}

	if err := state.Init(from, to, outputRoot); err != nil {
		return "", err
	}
	return outputRoot, nil
}

func estimateCandidate(candidate migrationCandidate, direction Direction) (MigrationFileRecord, []string) {
	record := MigrationFileRecord{
		InputPath:  candidate.RelPath,
		Type:       candidate.Type,
		Framework:  candidate.Framework,
		Status:     "estimated",
		Confidence: 0,
	}

	input, err := os.ReadFile(candidate.AbsPath)
	if err != nil {
		record.Status = "failed"
		record.Error = fmt.Sprintf("read source: %v", err)
		return record, []string{"read-source"}
	}

	output, err := convertCandidateContent(candidate, direction, string(input))
	if err != nil {
		record.Status = "failed"
		record.Error = err.Error()
		return record, []string{"conversion-error"}
	}
	if output == "" && candidate.IsConfig && !SupportsConfigConversion(direction.From, direction.To) {
		record.Status = "skipped"
		record.SkipReason = fmt.Sprintf("no Go-native config conversion for %s -> %s", direction.From, direction.To)
		return record, []string{"config-converter-missing"}
	}

	record.TodosAdded = countTODOs(output)
	record.Confidence = predictMigrationConfidence(output, candidate.Type, direction)
	record.Warnings = warningsFromOutput(output, candidate.Type, direction)
	return record, extractBlockers(output, record)
}

func migrateCandidate(candidate migrationCandidate, direction Direction, outputRoot string, strictValidate bool) (MigrationFileRecord, error) {
	record := MigrationFileRecord{
		InputPath: candidate.RelPath,
		Type:      candidate.Type,
		Framework: candidate.Framework,
	}
	if candidate.IsConfig {
		if !SupportsConfigConversion(direction.From, direction.To) {
			record.Status = "skipped"
			record.SkipReason = fmt.Sprintf("no Go-native config conversion for %s -> %s", direction.From, direction.To)
			return record, nil
		}
		targetName := TargetConfigFileName(direction.To, filepath.Base(candidate.RelPath))
		targetDir := filepath.Join(outputRoot, filepath.Dir(candidate.RelPath))
		outputPath := filepath.Join(targetDir, targetName)
		result, err := RunConfigMigration(candidate.AbsPath, ConfigMigrationOptions{
			From:           direction.From,
			To:             direction.To,
			Output:         outputPath,
			ValidateSyntax: strictValidate,
		})
		if err != nil {
			return record, err
		}
		record.Status = "converted"
		record.OutputPath = result.Output
		output, readErr := os.ReadFile(result.Output)
		if readErr == nil {
			record.TodosAdded = countTODOs(string(output))
			record.Confidence = predictMigrationConfidence(string(output), candidate.Type, direction)
			record.Warnings = warningsFromOutput(string(output), candidate.Type, direction)
		}
		if record.Confidence == 0 {
			record.Confidence = 95
		}
		return record, nil
	}

	targetDir := filepath.Join(outputRoot, filepath.Dir(candidate.RelPath))
	result, err := RunTestMigration(candidate.AbsPath, TestMigrationOptions{
		From:           direction.From,
		To:             direction.To,
		Output:         targetDir,
		ValidateSyntax: strictValidate,
	})
	if err != nil {
		return record, err
	}
	if result.Execution == nil {
		return record, fmt.Errorf("native test migration produced no execution result for %s", candidate.RelPath)
	}
	record.Status = "converted"
	if len(result.Execution.Files) > 0 {
		record.OutputPath = result.Execution.Files[0].OutputPath
	}
	if record.OutputPath != "" {
		output, readErr := os.ReadFile(record.OutputPath)
		if readErr == nil {
			record.TodosAdded = countTODOs(string(output))
			record.Confidence = predictMigrationConfidence(string(output), candidate.Type, direction)
			record.Warnings = warningsFromOutput(string(output), candidate.Type, direction)
		}
	}
	if record.Confidence == 0 {
		record.Confidence = 95
	}
	return record, nil
}

func discoverMigrationCandidates(root, from string) ([]migrationCandidate, error) {
	framework, ok := LookupFramework(from)
	if !ok {
		return nil, fmt.Errorf("unknown framework: %s", from)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	ignoredOutputRoot := migrationOutputRootToIgnore(rootAbs)

	candidates := make([]migrationCandidate, 0, 32)
	err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipIgnoredOutputDir(path, ignoredOutputRoot) {
				return filepath.SkipDir
			}
			if shouldSkipWorkflowDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)

		if configFramework := DetectConfigFramework(path); configFramework == from {
			candidates = append(candidates, migrationCandidate{
				AbsPath:    path,
				RelPath:    rel,
				Type:       "config",
				Framework:  configFramework,
				Language:   framework.Language,
				IsConfig:   true,
				ConfigName: filepath.Base(rel),
			})
			return nil
		}

		if !matchesFrameworkLanguage(path, framework.Language) {
			return nil
		}
		if !isLikelyTestPath(path) && !isLikelyHelperPath(path) {
			detection := analysis.DetectFrameworkForFile(path)
			if detection.Framework != from {
				return nil
			}
		}

		detection := analysis.DetectFrameworkForFile(path)
		fileType := "helper"
		if isLikelyHelperPath(path) {
			fileType = "helper"
		} else if isLikelyTestPath(path) || detection.Framework == from {
			fileType = "test"
		}
		candidates = append(candidates, migrationCandidate{
			AbsPath:   path,
			RelPath:   rel,
			Type:      fileType,
			Framework: detection.Framework,
			Language:  framework.Language,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan migration root: %w", err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Type == candidates[j].Type {
			return candidates[i].RelPath < candidates[j].RelPath
		}
		if candidates[i].Type == "config" {
			return false
		}
		if candidates[j].Type == "config" {
			return true
		}
		return candidates[i].RelPath < candidates[j].RelPath
	})
	return candidates, nil
}

func convertCandidateContent(candidate migrationCandidate, direction Direction, input string) (string, error) {
	if candidate.IsConfig {
		if !SupportsConfigConversion(direction.From, direction.To) {
			return "", nil
		}
		return ConvertConfig(input, direction.From, direction.To)
	}
	return convertSourceContent(direction, input)
}

func convertSourceContent(direction Direction, input string) (string, error) {
	switch direction.From + "-" + direction.To {
	case "cypress-playwright":
		return ConvertCypressToPlaywrightSource(input)
	case "cypress-selenium":
		return ConvertCypressToSeleniumSource(input)
	case "cypress-webdriverio":
		return ConvertCypressToWdioSource(input)
	case "jasmine-jest":
		return ConvertJasmineToJestSource(input)
	case "jest-jasmine":
		return ConvertJestToJasmineSource(input)
	case "jest-mocha":
		return ConvertJestToMochaSource(input)
	case "jest-vitest":
		return ConvertJestToVitestSource(input)
	case "junit4-junit5":
		return ConvertJUnit4ToJunit5Source(input)
	case "junit5-testng":
		return ConvertJUnit5ToTestNGSource(input)
	case "mocha-jest":
		return ConvertMochaToJestSource(input)
	case "nose2-pytest":
		return ConvertNose2ToPytestSource(input)
	case "playwright-cypress":
		return ConvertPlaywrightToCypressSource(input)
	case "playwright-puppeteer":
		return ConvertPlaywrightToPuppeteerSource(input)
	case "playwright-selenium":
		return ConvertPlaywrightToSeleniumSource(input)
	case "playwright-webdriverio":
		return ConvertPlaywrightToWdioSource(input)
	case "puppeteer-playwright":
		return ConvertPuppeteerToPlaywrightSource(input)
	case "pytest-unittest":
		return ConvertPytestToUnittestSource(input)
	case "selenium-cypress":
		return ConvertSeleniumToCypressSource(input)
	case "selenium-playwright":
		return ConvertSeleniumToPlaywrightSource(input)
	case "testcafe-cypress":
		return ConvertTestCafeToCypressSource(input)
	case "testcafe-playwright":
		return ConvertTestCafeToPlaywrightSource(input)
	case "testng-junit5":
		return ConvertTestNGToJunit5Source(input)
	case "unittest-pytest":
		return ConvertUnittestToPytestSource(input)
	case "webdriverio-cypress":
		return ConvertWdioToCypressSource(input)
	case "webdriverio-playwright":
		return ConvertWdioToPlaywrightSource(input)
	default:
		return "", fmt.Errorf("go-native conversion execution is not implemented for %s -> %s", direction.From, direction.To)
	}
}

func matchesFrameworkLanguage(path, language string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch language {
	case "javascript":
		return isJSExtForWorkflow(ext)
	case "python":
		return ext == ".py"
	case "java":
		return ext == ".java"
	default:
		return false
	}
}

func isJSExtForWorkflow(ext string) bool {
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func isLikelyHelperPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := strings.ToLower(filepath.Base(path))
	for _, fragment := range []string{"/support/", "/helpers/", "/helper/", "/fixtures/", "/mocks/", "/__mocks__/", "/setup/"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	for _, token := range []string{"helper", "helpers", "fixture", "fixtures", "setup", "support", "mock"} {
		if strings.Contains(base, token) {
			return true
		}
	}
	return false
}

func shouldSkipWorkflowDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "dist", "build", ".terrain", "vendor", "third_party", ".venv", "venv", "coverage":
		return true
	default:
		return false
	}
}

func accumulateEstimateSummary(summary *MigrationEstimateSummary, record MigrationFileRecord) {
	summary.TotalFiles++
	switch record.Type {
	case "test":
		summary.TestFiles++
	case "helper":
		summary.HelperFiles++
	case "config":
		summary.ConfigFiles++
	default:
		summary.OtherFiles++
	}
	switch {
	case record.Confidence >= 90:
		summary.PredictedHigh++
	case record.Confidence >= 70:
		summary.PredictedMedium++
	case record.Confidence > 0:
		summary.PredictedLow++
	}
}

func blockersFromCounts(counts map[string]int) []MigrationBlocker {
	items := make([]MigrationBlocker, 0, len(counts))
	for pattern, count := range counts {
		items = append(items, MigrationBlocker{Pattern: pattern, Count: count, Impact: "manual-review"})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Pattern < items[j].Pattern
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func estimateEffort(records []MigrationFileRecord) MigrationEffortEstimate {
	estimate := MigrationEffortEstimate{}
	for _, record := range records {
		switch {
		case record.Confidence > 0 && record.Confidence < 70:
			estimate.LowConfidenceFiles++
		case record.Confidence >= 70 && record.Confidence < 90:
			estimate.MediumConfidenceFiles++
		}
	}
	estimate.EstimatedManualMins = estimate.LowConfidenceFiles*15 + estimate.MediumConfidenceFiles*5
	switch {
	case estimate.EstimatedManualMins == 0:
		estimate.Description = "Fully automated — no manual intervention expected"
	case estimate.EstimatedManualMins < 30:
		estimate.Description = "Minimal manual effort expected"
	case estimate.EstimatedManualMins < 120:
		estimate.Description = "Moderate manual effort expected"
	default:
		estimate.Description = "Significant manual effort expected"
	}
	return estimate
}

func predictMigrationConfidence(output, fileType string, direction Direction) int {
	base := 95
	if fileType == "config" {
		base = 80
	}
	todos := countTODOs(output)
	warnings := countWarnings(output, direction)
	confidence := base
	switch {
	case todos == 0:
		confidence = base
	case todos == 1:
		confidence = minInt(base, 80)
	case todos <= 3:
		confidence = minInt(base, 70)
	default:
		confidence = minInt(base, 55)
	}
	switch {
	case warnings == 0:
	case warnings == 1:
		confidence = minInt(confidence, base-10)
	default:
		confidence = minInt(confidence, base-20)
	}
	if strings.Contains(output, "UNCONVERTIBLE") {
		confidence = minInt(confidence, 60)
	}
	if warning := semanticValidationWarning(direction, output); warning != "" {
		confidence = minInt(confidence, 60)
	}
	return confidence
}

func warningsFromOutput(output, fileType string, direction Direction) []string {
	warnings := make([]string, 0, 4)
	todos := countTODOs(output)
	if todos > 0 {
		warnings = append(warnings, fmt.Sprintf("%d manual follow-up comment(s) inserted", todos))
	}
	for _, warning := range extractTerrainWarnings(output) {
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	if strings.Contains(output, "UNCONVERTIBLE") {
		warnings = append(warnings, "contains unconvertible output markers")
	}
	warnings = append(warnings, semanticValidationWarnings(direction, output)...)
	if fileType == "config" && todos == 0 && len(warnings) == 0 {
		return warnings
	}
	return dedupeStrings(warnings)
}

func countWarnings(output string, direction Direction) int {
	return len(extractTerrainWarnings(output)) + len(semanticValidationWarnings(direction, output))
}

func extractTerrainWarnings(output string) []string {
	lines := strings.Split(output, "\n")
	warnings := make([]string, 0, 2)
	for _, line := range lines {
		if !strings.Contains(line, "TERRAIN-WARNING") {
			continue
		}
		message := strings.TrimSpace(line)
		if idx := strings.Index(message, "TERRAIN-WARNING:"); idx >= 0 {
			message = strings.TrimSpace(message[idx+len("TERRAIN-WARNING:"):])
		}
		if message != "" {
			warnings = append(warnings, message)
		}
	}
	return warnings
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		deduped = append(deduped, value)
	}
	return deduped
}

func countTODOs(output string) int {
	return strings.Count(output, "TERRAIN-TODO")
}

func extractBlockers(output string, record MigrationFileRecord) []string {
	if record.Status == "skipped" && record.SkipReason != "" {
		return []string{record.SkipReason}
	}
	if record.Status == "failed" && record.Error != "" {
		return []string{record.Error}
	}
	lines := strings.Split(output, "\n")
	blockers := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, line := range lines {
		if !strings.Contains(line, "TERRAIN-TODO") {
			continue
		}
		pattern := strings.TrimSpace(line)
		if idx := strings.Index(pattern, "TERRAIN-TODO:"); idx >= 0 {
			pattern = strings.TrimSpace(pattern[idx+len("TERRAIN-TODO:"):])
		} else if idx := strings.Index(pattern, "TERRAIN-TODO"); idx >= 0 {
			pattern = strings.TrimSpace(pattern[idx+len("TERRAIN-TODO"):])
		}
		pattern = strings.TrimLeft(pattern, ":- ")
		if open := strings.Index(pattern, "["); open >= 0 {
			if close := strings.Index(pattern[open:], "]"); close > 0 {
				pattern = pattern[open+1 : open+close]
			}
		}
		if pattern == "" || seen[pattern] {
			continue
		}
		seen[pattern] = true
		blockers = append(blockers, pattern)
	}
	return blockers
}

func countRecordsByConfidence(records []MigrationFileRecord, min, max int) int {
	total := 0
	for _, record := range records {
		if record.Confidence >= min && record.Confidence < max {
			total++
		}
	}
	return total
}

func countManualRecords(records []MigrationFileRecord) int {
	total := 0
	for _, record := range records {
		if record.Status == "failed" || record.Status == "skipped" || record.Confidence == 0 {
			total++
		}
	}
	return total
}

func doctorPathCheck(root string) MigrationDoctorCheck {
	info, err := os.Stat(root)
	if err != nil {
		return MigrationDoctorCheck{
			ID:          "target-path",
			Label:       "Target path",
			Status:      "FAIL",
			Detail:      fmt.Sprintf("%s does not exist", root),
			Remediation: "Provide a valid directory path",
		}
	}
	if !info.IsDir() {
		return MigrationDoctorCheck{
			ID:          "target-path",
			Label:       "Target path",
			Status:      "FAIL",
			Detail:      fmt.Sprintf("%s is not a directory", root),
			Remediation: "Provide a directory, not a file",
		}
	}
	return MigrationDoctorCheck{
		ID:     "target-path",
		Label:  "Target path",
		Status: "PASS",
		Detail: root,
	}
}

func doctorWritableCheck(root string) MigrationDoctorCheck {
	probe, err := os.CreateTemp(root, ".terrain-doctor-*")
	if err != nil {
		return MigrationDoctorCheck{
			ID:          "output-permissions",
			Label:       "Output permissions",
			Status:      "WARN",
			Detail:      "Directory is not writable",
			Remediation: "Check write permissions on the target directory",
		}
	}
	name := probe.Name()
	probe.Close()
	_ = os.Remove(name)
	return MigrationDoctorCheck{
		ID:     "output-permissions",
		Label:  "Output permissions",
		Status: "PASS",
		Detail: "Directory is writable",
	}
}

func doctorProjectConfigCheck(root string) MigrationDoctorCheck {
	found := make([]string, 0, 4)
	for _, name := range []string{"package.json", "pyproject.toml", "setup.cfg", "pom.xml", "build.gradle", "build.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			found = append(found, name)
		}
	}
	if len(found) == 0 {
		return MigrationDoctorCheck{
			ID:          "project-config",
			Label:       "Project config",
			Status:      "WARN",
			Detail:      "No package or build configuration detected",
			Remediation: "Add or point Terrain at the project root with package.json, pyproject.toml, pom.xml, or build.gradle",
		}
	}
	return MigrationDoctorCheck{
		ID:      "project-config",
		Label:   "Project config",
		Status:  "PASS",
		Detail:  "Detected: " + strings.Join(found, ", "),
		Verbose: fmt.Sprintf("%d project configuration file(s) detected", len(found)),
	}
}

func doctorTestInventoryCheck(root string) MigrationDoctorCheck {
	counts, total := scanWorkflowFrameworkInventory(root)
	if total == 0 {
		return MigrationDoctorCheck{
			ID:          "test-files",
			Label:       "Test files",
			Status:      "WARN",
			Detail:      "No test files found",
			Remediation: "Point Terrain at a test-containing directory or add tests before migrating",
		}
	}
	summary := make([]string, 0, len(counts))
	for framework, count := range counts {
		summary = append(summary, fmt.Sprintf("%s (%d)", framework, count))
	}
	sort.Strings(summary)
	return MigrationDoctorCheck{
		ID:      "test-files",
		Label:   "Test files",
		Status:  "PASS",
		Detail:  fmt.Sprintf("%d test file(s) found", total),
		Verbose: strings.Join(summary, ", "),
	}
}

func doctorSupportedDirectionsCheck(root string) MigrationDoctorCheck {
	counts, _ := scanWorkflowFrameworkInventory(root)
	dominant := dominantFramework(counts)
	if dominant == "" {
		return MigrationDoctorCheck{
			ID:          "supported-directions",
			Label:       "Supported directions",
			Status:      "WARN",
			Detail:      "Could not determine a dominant framework",
			Remediation: "Run `terrain detect <path>` or pass an explicit --from framework during migration",
		}
	}
	targets := SupportedTargets(dominant)
	if len(targets) == 0 {
		return MigrationDoctorCheck{
			ID:          "supported-directions",
			Label:       "Supported directions",
			Status:      "WARN",
			Detail:      fmt.Sprintf("No conversion targets cataloged for %s", dominant),
			Remediation: "Choose a supported source framework from `terrain list-conversions`",
		}
	}
	return MigrationDoctorCheck{
		ID:      "supported-directions",
		Label:   "Supported directions",
		Status:  "PASS",
		Detail:  fmt.Sprintf("%s can migrate to: %s", dominant, strings.Join(targets, ", ")),
		Verbose: fmt.Sprintf("Detected from %d test file(s)", counts[dominant]),
	}
}

func doctorMigrationStateCheck(root string) MigrationDoctorCheck {
	state := NewMigrationStateManager(root)
	exists, err := state.Exists()
	if err != nil {
		return MigrationDoctorCheck{
			ID:          "migration-state",
			Label:       "Migration state",
			Status:      "WARN",
			Detail:      "Could not inspect migration state",
			Remediation: err.Error(),
		}
	}
	if !exists {
		return MigrationDoctorCheck{
			ID:     "migration-state",
			Label:  "Migration state",
			Status: "PASS",
			Detail: "No active migration state",
		}
	}
	if err := state.Load(); err != nil {
		return MigrationDoctorCheck{
			ID:          "migration-state",
			Label:       "Migration state",
			Status:      "WARN",
			Detail:      "Migration state exists but could not be loaded",
			Remediation: err.Error(),
		}
	}
	status := state.Status()
	return MigrationDoctorCheck{
		ID:      "migration-state",
		Label:   "Migration state",
		Status:  "PASS",
		Detail:  fmt.Sprintf("%d tracked file(s): %d converted, %d failed, %d skipped", status.Total, status.Converted, status.Failed, status.Skipped),
		Verbose: fmt.Sprintf("State root: %s", state.statePath),
	}
}

func summarizeDoctorChecks(checks []MigrationDoctorCheck) MigrationDoctorResult {
	result := MigrationDoctorResult{Checks: checks}
	for _, check := range checks {
		switch check.Status {
		case "PASS":
			result.Summary.Pass++
		case "WARN":
			result.Summary.Warn++
		case "FAIL":
			result.Summary.Fail++
		}
	}
	result.Summary.Total = len(checks)
	result.HasFail = result.Summary.Fail > 0
	return result
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func migrationOutputRootToIgnore(root string) string {
	state := NewMigrationStateManager(root)
	exists, err := state.Exists()
	if err != nil || !exists {
		return ""
	}
	if err := state.Load(); err != nil || state.state == nil {
		return ""
	}
	outputRoot := filepath.Clean(state.state.OutputRoot)
	if outputRoot == "" || outputRoot == root {
		return ""
	}
	prefix := root + string(os.PathSeparator)
	if strings.HasPrefix(outputRoot, prefix) {
		return outputRoot
	}
	return ""
}

func shouldSkipIgnoredOutputDir(path, ignoredOutputRoot string) bool {
	if ignoredOutputRoot == "" {
		return false
	}
	path = filepath.Clean(path)
	return path == ignoredOutputRoot
}

func scanWorkflowFrameworkInventory(root string) (map[string]int, int) {
	counts := map[string]int{}
	total := 0
	ignoredOutputRoot := migrationOutputRootToIgnore(root)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipIgnoredOutputDir(path, ignoredOutputRoot) {
				return filepath.SkipDir
			}
			if shouldSkipWorkflowDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isLikelyTestPath(path) {
			return nil
		}
		total++
		result := analysis.DetectFrameworkForFile(path)
		if result.Framework != "" && result.Framework != "unknown" {
			counts[result.Framework]++
		}
		return nil
	})
	return counts, total
}

func dominantFramework(counts map[string]int) string {
	best := ""
	bestCount := 0
	for framework, count := range counts {
		if count > bestCount || (count == bestCount && framework < best) {
			best = framework
			bestCount = count
		}
	}
	return best
}
