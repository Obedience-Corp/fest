package workflow

import (
	"errors"
	"fmt"
)

// Package-level errors for workflow operations.
var (
	// ErrNoSchema is returned when no .workflow.yaml file is found.
	ErrNoSchema = errors.New("no workflow schema found")

	// ErrSchemaExists is returned when trying to init an already-initialized workflow.
	ErrSchemaExists = errors.New("workflow already initialized")

	// ErrInvalidSchema is returned when the schema file is malformed.
	ErrInvalidSchema = errors.New("invalid workflow schema")

	// ErrInvalidTransition is returned when a move violates transition rules.
	ErrInvalidTransition = errors.New("transition not allowed")

	// ErrItemNotFound is returned when an item doesn't exist.
	ErrItemNotFound = errors.New("item not found")

	// ErrInvalidStatus is returned when a status path is invalid.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrStatusNotFound is returned when a status directory doesn't exist.
	ErrStatusNotFound = errors.New("status not found")

	// ErrAlreadyExists is returned when an item already exists at the destination.
	ErrAlreadyExists = errors.New("item already exists at destination")

	// ErrFlowNested is returned when trying to create a flow inside another flow.
	ErrFlowNested = errors.New("cannot create flow inside existing flow")
)

// FlowNestedError provides details about the parent flow that prevents nesting.
type FlowNestedError struct {
	ParentSchemaPath string
}

func (e *FlowNestedError) Error() string {
	return fmt.Sprintf("%s\n\nFound parent flow at: %s\n\nFlows cannot be nested because:\n  - Path resolution becomes ambiguous\n  - Active work tracking is complicated\n  - Status directories would conflict\n\nTo create a new flow, navigate outside the current flow first.",
		ErrFlowNested, e.ParentSchemaPath)
}

func (e *FlowNestedError) Unwrap() error {
	return ErrFlowNested
}
