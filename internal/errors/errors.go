// Package errors provides structured error types for fest CLI.
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrAlreadyPrinted is a sentinel error indicating the message was already
// printed to the user. main.go should skip re-printing but still exit non-zero.
var ErrAlreadyPrinted = fmt.Errorf("already printed")

// Error codes for categorization.
const (
	ErrCodeNotFound   = "NOT_FOUND"
	ErrCodeValidation = "VALIDATION"
	ErrCodeIO         = "IO"
	ErrCodeConfig     = "CONFIG"
	ErrCodeTemplate   = "TEMPLATE"
	ErrCodeParse      = "PARSE"
	ErrCodeInternal   = "INTERNAL"
	ErrCodePermission = "PERMISSION"
	ErrCodeNetwork    = "NETWORK"
)

// Standard hints for common error scenarios.
const (
	HintFestivalNotFound      = "Navigate to a festival directory or run 'fest list --all' to see available festivals"
	HintPhaseNotFound         = "Run 'fest status list --type phase' to see available phases"
	HintSequenceNotFound      = "Run 'fest status list --type sequence' to see available sequences"
	HintCreateFestival        = "Run 'fest create festival' or 'fest tui' to create a new festival"
	HintCheckPath             = "Check the path and try again, or run from a different directory"
	HintCheckConfig           = "Check your fest.yaml configuration for syntax errors"
	HintCheckTemplate         = "Run 'fest validate' to check for template issues"
	HintRunInit               = "Run 'fest init' to initialize a festival workspace"
	HintCheckPermissions      = "Check file/directory permissions and try again"
	HintCheckNetwork          = "Check that the remote is reachable from this machine, then try again"
	HintCheckTLS              = "The remote was reached but its certificate could not be verified. Install CA certificates (e.g. the ca-certificates package) or set GIT_SSL_CAINFO to a CA bundle"
	HintCheckDNS              = "The hostname did not resolve. Check DNS and your network connection"
	HintCheckAuth             = "The remote refused access. Check credentials, or use a public URL if the repository is public"
	HintUseForce              = "Use --force to skip confirmation prompts"
	HintNavigateToFestival    = "Navigate to a festival directory first"
	HintUseInteractiveMode    = "Use 'fest tui' for interactive mode"
	HintCheckStatus           = "Run 'fest status' to see current location and status"
	HintSeeHelp               = "Run 'fest help' for more information"
	HintUnderstandMethodology = "Run 'fest understand methodology' to learn about the festival structure"
)

// Error is a structured error type with code, context, and chain support.
type Error struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Op      string                 `json:"op,omitempty"`
	Err     error                  `json:"-"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
	Hint    string                 `json:"hint,omitempty"` // actionable suggestion

	// hintFromClass marks a Hint that a constructor attached from the error
	// class alone, with no knowledge of what actually failed. Those lose to any
	// hint chosen for the specific failure, wherever it sits in the chain.
	// Unexported so it cannot be set from outside: WithHint is how a caller
	// says "this one is mine".
	hintFromClass bool
}

// Error returns the error string with context, followed by exactly one hint.
//
// Which hint that is, is resolveHint's decision; see it for the precedence and
// why. Inner hints are deliberately not printed: rendering a cause with %v
// would splice its "Hint:" line into the middle of this one's message, so a
// two-layer error printed two hints and a three-layer error printed three.
func (e *Error) Error() string {
	msg := e.chainMessage()
	if hint := e.resolveHint(); hint != "" {
		msg = fmt.Sprintf("%s\nHint: %s", msg, hint)
	}
	return msg
}

// chainMessage renders this error and its causes with every nested "Hint:" line
// removed, so only Error() decides which single hint is shown.
//
// It strips the rendered text rather than recursing into nested *Error values.
// Recursion via errors.As looked equivalent and was not: errors.As searches the
// whole chain, so a plain fmt.Errorf sitting between two *Error layers was
// skipped and its message silently vanished from the output. Stripping works
// for any chain shape and cannot drop a layer.
func (e *Error) chainMessage() string {
	cause := ""
	if e.Err != nil {
		cause = stripHintLines(e.Err.Error())
	}

	switch {
	case e.Op != "" && cause != "":
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Message, cause)
	case e.Op != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	case cause != "":
		return fmt.Sprintf("%s: %s", e.Message, cause)
	default:
		return e.Message
	}
}

// stripHintLines removes rendered "Hint: ..." lines from a cause's text.
func stripHintLines(msg string) string {
	if !strings.Contains(msg, "\nHint: ") {
		return msg
	}
	lines := strings.Split(msg, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "Hint: ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// resolveHint returns the hint to show: the innermost hint that says something
// about this particular failure, falling back to a class-level hint only when
// no layer offers better, and outward when a layer carries none.
//
// Innermost wins because that layer knows what actually failed, while outer
// layers only know what was being attempted. A TLS verification failure inside
// 'fest init' is the worked example: the inner layer can say "install CA
// certificates", and the outer one can only offer "check your internet
// connection", which is actively misleading on a machine whose network is fine.
// The outer context is not lost, because the message chain already carries it.
//
// Specific beats inner, though, or innermost-wins inverts its own purpose.
// Several constructors attach a catch-all hint from the error class alone —
// Validation hands out "Run 'fest help'" to every caller — so a chain whose
// innermost layer happens to be a Validation would surface that over an outer
// hint written for the actual operation. Real case: a sync failure reported
// "Run 'fest help' for more information" instead of the connection advice the
// call site attached, which is the same "check file permissions" dead end that
// visible hints were meant to remove.
func (e *Error) resolveHint() string {
	if h := e.specificHint(); h != "" {
		return h
	}
	return e.fallbackHint()
}

// specificHint returns the innermost hint that was chosen for this failure
// rather than for its class: an explicit WithHint, or one a constructor derived
// from the failure itself.
func (e *Error) specificHint() string {
	var inner *Error
	if e.Err != nil && errors.As(e.Err, &inner) {
		if h := inner.specificHint(); h != "" {
			return h
		}
	}
	if e.hintFromClass {
		return ""
	}
	return e.Hint
}

// fallbackHint returns the innermost class-level hint, used only when nothing
// in the chain offers something more specific.
func (e *Error) fallbackHint() string {
	var inner *Error
	if e.Err != nil && errors.As(e.Err, &inner) {
		if h := inner.fallbackHint(); h != "" {
			return h
		}
	}
	return e.Hint
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error {
	return e.Err
}

// MarshalJSON implements json.Marshaler with full error chain.
func (e *Error) MarshalJSON() ([]byte, error) {
	type errorJSON struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Op      string                 `json:"op,omitempty"`
		Cause   string                 `json:"cause,omitempty"`
		Hint    string                 `json:"hint,omitempty"`
		Fields  map[string]interface{} `json:"fields,omitempty"`
	}

	ej := errorJSON{
		Code:    e.Code,
		Message: e.Message,
		Op:      e.Op,
		Hint:    e.Hint,
		Fields:  e.Fields,
	}
	if e.Err != nil {
		ej.Cause = e.Err.Error()
	}

	return json.Marshal(ej)
}

// New creates a new error with a message.
func New(message string) *Error {
	return &Error{
		Code:    ErrCodeInternal,
		Message: message,
		Fields:  make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with a message.
func Wrap(err error, message string) *Error {
	return &Error{
		Code:    ErrCodeInternal,
		Message: message,
		Err:     err,
		Fields:  make(map[string]interface{}),
	}
}

// Wrapf wraps an existing error with a formatted message.
func Wrapf(err error, format string, args ...interface{}) *Error {
	return &Error{
		Code:    ErrCodeInternal,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
		Fields:  make(map[string]interface{}),
	}
}

// WithCode sets the error code.
func (e *Error) WithCode(code string) *Error {
	e.Code = code
	return e
}

// WithOp sets the operation name.
func (e *Error) WithOp(op string) *Error {
	e.Op = op
	return e
}

// WithField adds a context field.
func (e *Error) WithField(key string, value interface{}) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	e.Fields[key] = value
	return e
}

// WithFields adds multiple context fields.
func (e *Error) WithFields(fields map[string]interface{}) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

// WithHint adds an actionable suggestion to the error.
// A hint set here is about this call site, so it outranks the catch-all a
// constructor may have attached from the error class — including one further in
// the chain.
func (e *Error) WithHint(hint string) *Error {
	e.Hint = hint
	e.hintFromClass = false
	return e
}

// WithHintf adds a formatted actionable suggestion to the error.
func (e *Error) WithHintf(format string, args ...interface{}) *Error {
	e.Hint = fmt.Sprintf(format, args...)
	e.hintFromClass = false
	return e
}

// NotFound creates a NOT_FOUND error with context-aware hints.
func NotFound(resource string) *Error {
	return &Error{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Fields:  map[string]interface{}{"resource": resource},
		Hint:    hintForResource(resource),
	}
}

// hintForResource returns an appropriate hint based on the resource type.
func hintForResource(resource string) string {
	switch resource {
	case "festival":
		return HintFestivalNotFound
	case "phase":
		return HintPhaseNotFound
	case "sequence":
		return HintSequenceNotFound
	default:
		return HintCheckPath
	}
}

// Validation creates a VALIDATION error with a helpful hint.
func Validation(message string) *Error {
	return &Error{
		Code:          ErrCodeValidation,
		Message:       message,
		Fields:        make(map[string]interface{}),
		Hint:          HintSeeHelp,
		hintFromClass: true,
	}
}

// IO creates an IO error with permission check hint.
func IO(op string, err error) *Error {
	return &Error{
		Code:          ErrCodeIO,
		Message:       "I/O operation failed",
		Op:            op,
		Err:           err,
		Fields:        make(map[string]interface{}),
		Hint:          HintCheckPermissions,
		hintFromClass: true,
	}
}

// Network creates a NETWORK error for an operation that failed to reach a
// remote. It exists so remote failures stop being reported as IO errors, whose
// hint tells the operator to check file permissions: on a machine that simply
// has no route to GitHub, that hint sends them to the one place the problem is
// not. detail carries the remote's own message (git's stderr, say), which is
// where the real cause lives.
func Network(op string, err error, detail string) *Error {
	message := "could not reach the remote"
	if d := strings.TrimSpace(detail); d != "" {
		message = message + ": " + d
	}
	return &Error{
		Code:    ErrCodeNetwork,
		Message: message,
		Op:      op,
		Err:     err,
		Fields:  make(map[string]interface{}),
		Hint:    networkHint(detail),
	}
}

// networkHint picks the hint that matches what the remote actually said.
//
// "check your connection" is the right advice for exactly one of these causes.
// A machine with a working network and no CA bundle, which is the normal state
// on minimal and embedded systems, fails TLS verification and needs to be told
// about certificates, not about its connection. Sending someone to debug a
// network that is already up costs them the whole session.
func networkHint(detail string) string {
	d := strings.ToLower(detail)
	switch {
	case containsAny(d, "certificate", "ssl", "tls", "cafile", "self-signed", "self signed"):
		return HintCheckTLS
	case containsAny(d, "could not resolve host", "name or service not known", "temporary failure in name resolution"):
		return HintCheckDNS
	case containsAny(d, "authentication failed", "permission denied", "could not read username",
		"could not read password", "access denied", "403", "terminal prompts disabled"):
		return HintCheckAuth
	default:
		return HintCheckNetwork
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// Config creates a CONFIG error with configuration check hint.
func Config(message string) *Error {
	return &Error{
		Code:          ErrCodeConfig,
		Message:       message,
		Fields:        make(map[string]interface{}),
		Hint:          HintCheckConfig,
		hintFromClass: true,
	}
}

// Template creates a TEMPLATE error with template check hint.
func Template(message string) *Error {
	return &Error{
		Code:          ErrCodeTemplate,
		Message:       message,
		Fields:        make(map[string]interface{}),
		Hint:          HintCheckTemplate,
		hintFromClass: true,
	}
}

// Parse creates a PARSE error with configuration check hint.
func Parse(message string, err error) *Error {
	return &Error{
		Code:          ErrCodeParse,
		Message:       message,
		Err:           err,
		Fields:        make(map[string]interface{}),
		Hint:          HintCheckConfig,
		hintFromClass: true,
	}
}

// Code extracts the error code from any error.
// Returns ErrCodeInternal if error is not a structured Error.
func Code(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ErrCodeInternal
}

// Is checks if the error has the given code.
func Is(err error, code string) bool {
	return Code(err) == code
}
