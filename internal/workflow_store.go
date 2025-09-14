package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Workflow state structures
type WorkflowEntry struct {
	TicketID     string    `json:"ticketId"`
	Branch       string    `json:"branch"`
	PrURL        string    `json:"prUrl"`
	Status       string    `json:"status"`
	Dependencies []string  `json:"dependencies"`
	LastUpdated  time.Time `json:"lastUpdated"`
}

type WorkflowState map[string]WorkflowEntry

// loadWorkflowState loads the workflow state from the state file
func loadWorkflowState() (WorkflowState, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	stateFile := filepath.Join(homeDir, ".dev_workflow_state.json")

	state := make(WorkflowState)
	if data, err := os.ReadFile(stateFile); err == nil {
		// Try to unmarshal as map first (new format)
		err = json.Unmarshal(data, &state)
		if err != nil {
			// If that fails, try to unmarshal as array (old format) for backward compatibility
			var oldState []WorkflowEntry
			if err := json.Unmarshal(data, &oldState); err != nil {
				return nil, err
			}
			// Convert old format to new format
			state = make(WorkflowState)
			for _, entry := range oldState {
				state[entry.TicketID] = entry
			}
		}
	}
	return state, nil
}

func storeWorkflowStatus(ticketID, branchName string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stateFile := filepath.Join(homeDir, ".dev_workflow_state.json")

	state := make(WorkflowState)
	if data, err := os.ReadFile(stateFile); err == nil {
		// Try to unmarshal as map first (new format)
		err = json.Unmarshal(data, &state)
		if err != nil {
			// If that fails, try to unmarshal as array (old format) for backward compatibility
			var oldState []WorkflowEntry
			if err := json.Unmarshal(data, &oldState); err != nil {
				return err
			}
			// Convert old format to new format
			state = make(WorkflowState)
			for _, entry := range oldState {
				state[entry.TicketID] = entry
			}
		}
	}

	// Add new entry
	entry := WorkflowEntry{
		TicketID:     ticketID,
		Branch:       branchName,
		Status:       "in-progress",
		Dependencies: []string{},
		LastUpdated:  time.Now(),
	}
	state[ticketID] = entry

	// Write back (always in new map format)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

func deleteWorkflowState(ticketID string) error {
	state, err := loadWorkflowState()
	if err != nil {
		return err
	}
	delete(state, ticketID)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stateFile := filepath.Join(homeDir, ".dev_workflow_state.json")
	return os.WriteFile(stateFile, data, 0644)
}

func workflowStateForBranch(branch string) (WorkflowEntry, bool, error) {
	wfs, err := loadWorkflowState()
	if err != nil {
		return WorkflowEntry{}, false, fmt.Errorf("loading workflow state: %s", err)
	}

	var found bool
	var wfe WorkflowEntry
	for _, w := range wfs {
		found = w.Branch == branch
		if found {
			wfe = w
			break
		}
	}

	return wfe, found, nil
}

// buildWorkflowOptions builds the options list and lookup map for workflow entries and branches
func buildWorkflowOptions(state WorkflowState) ([]string, map[string]string) {
	var options []string
	optionMap := make(map[string]string)

	// Add workflow state entries first
	for _, entry := range state {
		option := fmt.Sprintf("🎫 %s (%s)", entry.Branch, entry.TicketID)
		options = append(options, option)
		optionMap[option] = entry.Branch
	}

	return options, optionMap
}

// buildCompleteOptions builds the options list and lookup map for completing workflows
func buildCompleteOptions(currentBranch string, state WorkflowState) ([]string, map[string]string) {
	var options []string
	optionMap := make(map[string]string)

	var currentOption string
	hasWorkflow := false

	// Collect all options in one pass
	for _, entry := range state {
		if entry.Branch == currentBranch {
			currentOption = fmt.Sprintf("🎫 %s (%s) [current branch]", entry.Branch, entry.TicketID)
			hasWorkflow = true
		} else {
			option := fmt.Sprintf("🎫 %s (%s)", entry.Branch, entry.TicketID)
			options = append(options, option)
			optionMap[option] = entry.Branch
		}
	}

	// Prepend current branch
	if hasWorkflow {
		options = append([]string{currentOption}, options...)
		optionMap[currentOption] = currentBranch
	} else {
		currentOption = fmt.Sprintf("🌿 %s", currentBranch)
		options = append([]string{currentOption}, options...)
		optionMap[currentOption] = currentBranch
	}

	return options, optionMap
}
