# Feature: Template Variable Validation

**Description:**
Enhances the existing Jinja2-based template system with validation, error checking, and fail-fast mechanisms to prevent unreplaced template variables from executing in production.

## Current State

**Already Using Jinja2:** ✅
- `generate_boot_scripts.py` already uses Jinja2 `Environment` and `FileSystemLoader`
- `boot.sh` template uses Jinja2 syntax: `{{ TAILSCALE_KEY }}`
- Variables are properly rendered via `template.render()`

**Remaining Issues:**
- ❌ No validation that all template variables were replaced
- ❌ No detection of leftover `{{ VAR }}` patterns after rendering
- ❌ No fail-fast if required variables are missing
- ❌ Silent failures possible with literal template strings in output

## 🧩 Interface

### Enhanced Functions

**File:** `generate_boot_scripts.py`

**Existing Functions (Enhanced):**

| Function | Enhancement | Description |
|----------|-------------|-------------|
| `generate_shell_script()` | Add validation | Validates all variables replaced, returns bool + rendered content |
| `generate_cloud_init_yaml()` | Add validation | Validates shell script before cloud-init wrapping |

### New Validation Function

```python
def validate_rendered_template(rendered_content: str) -> Tuple[bool, List[str]]:
    """
    Validates that no unreplaced template variables remain in rendered content.

    Args:
        rendered_content: The rendered template string to validate

    Returns:
        Tuple of (is_valid, list_of_unreplaced_variables)

    Raises:
        TemplateValidationError: If unreplaced variables found
    """
```

## 📦 Dependencies

- Jinja2 (already in use)
- regex (standard library)

## 🧪 Tests

- [ ] Test: Template with all variables replaced passes validation
- [ ] Test: Template with missing variable fails validation
- [ ] Test: Template with partial variable syntax detected
- [ ] Test: Validation catches malformed `{{ VAR` (unclosed)
- [ ] Test: Validation allows legitimate `{{` uses (escaped, comments)
- [ ] Test: Error message includes list of unreplaced variables

## 📝 Design

### Problem Statement

The boot script template system uses Jinja2 for variable substitution. However, if a required variable is missing during rendering:
1. Jinja2 **does not raise an error** by default (undefined variables render as empty string)
2. Literal `{{ VAR }}` strings may remain in the output
3. Script executes with empty/unreplaced values, causing cryptic failures
4. Hard to debug - failures occur at runtime, not during provisioning

### Solution Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Template Rendering with Validation                          │
│                                                              │
│  1. Load Jinja2 template                                     │
│  2. Render with provided variables                          │
│  3. **NEW:** Validate rendered content                      │
│  4. **NEW:** Fail fast if unreplaced variables detected     │
│  5. Return validated script                                 │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Validation Logic                                           │
│                                                              │
│  Patterns to detect:                                         │
│  - `{{ VARIABLE_NAME }}`  - Unreplaced Jinja2 variable      │
│  - `{{ VARIABLE_NAME`     - Unclosed variable syntax        │
│  - `{{  `                 - Empty/malformed syntax          │
│                                                              │
│  Exclude (false positives):                                  │
│  - `{{ "literal" }}`     - Jinja2 string literals           │
│  - `{# comment #}`        - Jinja2 comments                  │
│  - `{% ... %}`            - Jinja2 statements                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Strategy

#### Option 1: Strict Undefined Mode (Recommended)

Set Jinja2 `undefined` parameter to raise error on undefined variables:

```python
from jinja2 import StrictUndefined

env = Environment(
    loader=FileSystemLoader(script_dir),
    undefined=StrictUndefined  # Fail fast on undefined variables
)
```

**Pros:**
- Simple, built-in Jinja2 feature
- Catches missing variables at render time
- Clear error messages

**Cons:**
- Requires all variables to be provided (even optional ones)
- May break existing code that relies on empty string defaults

#### Option 2: Post-Render Validation (Recommended as Complement)

After rendering, scan for leftover template syntax:

```python
import re

def validate_rendered_template(content: str) -> Tuple[bool, List[str]]:
    """Check for unreplaced template variables."""

    # Pattern for unreplaced Jinja2 variables
    # Matches {{ VAR }} but not {{ "literal" }} or {% ... %}
    pattern = r'\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}'

    matches = re.findall(pattern, content)

    if matches:
        unreplaced = list(set(matches))  # Unique variables
        return False, unreplaced

    return True, []
```

**Pros:**
- Catches any leftover template syntax
- Works regardless of Jinja2 settings
- Provides list of unreplaced variables for debugging

**Cons:**
- False positives possible (e.g., legitimate `{{` in strings)
- Additional regex processing step

#### Option 3: Hybrid Approach (Best Practice)

Combine both options for maximum safety:

1. Use `StrictUndefined` to catch missing variables at render time
2. Add post-render validation as safety net
3. Provide escape mechanism for edge cases

```python
from jinja2 import StrictUndefined

# Configure Jinja2 environment
env = Environment(
    loader=FileSystemLoader(script_dir),
    undefined=StrictUndefined,
    autoescape=False  # We're generating shell scripts, not HTML
)

# Render (will raise UndefinedError if variable missing)
rendered = template.render(**variables)

# Validate as safety net
is_valid, unreplaced = validate_rendered_template(rendered)
if not is_valid:
    raise TemplateValidationError(
        f"Unreplaced variables in rendered template: {', '.join(unreplaced)}"
    )
```

### Validation Patterns

**Detection Pattern:**
```regex
\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}
```
Matches: `{{ TAILSCALE_KEY }}`, `{{LEADER_IP}}`
Does NOT match: `{{ "literal" }}`, `{# comment #}`, `{% if %}`

**Examples of Caught Issues:**

| Rendered Output | Issue | Detected? |
|-----------------|-------|-----------|
| `TAILSCALE_KEY=""` | Variable provided but empty | ✅ Valid (empty is OK) |
| `TAILSCALE_KEY="{{ TAILSCALE_KEY }}"` | Variable not replaced | ❌ Invalid |
| `LEADER_IP="{{ LEADER_IP }}"` | Variable not replaced | ❌ Invalid |
| `ROLE="{{ ROLE }}"` | Variable not replaced | ❌ Invalid |
| `echo "{{ "{{ }}"` | Escaped braces | ✅ Valid (handled) |

### Error Handling

**TemplateValidationError Exception:**

```python
class TemplateValidationError(Exception):
    """Raised when template validation fails."""

    def __init__(self, message: str, unreplaced_variables: List[str]):
        self.unreplaced_variables = unreplaced_variables
        super().__init__(message)

    def __str__(self):
        vars_str = ", ".join(self.unreplaced_variables)
        return f"Template validation failed. Unreplaced variables: {vars_str}"
```

**User-Facing Error Messages:**

```
ERROR: Template validation failed
Unreplaced variables detected in rendered script:
  - TAILSCALE_KEY
  - LEADER_IP

This indicates that required template variables were not provided.
Check that all required variables are passed to template.render().

Context: generate_shell_script() for role='server'
File: src/infrastructure/boot_consul_nomad/boot.sh
```

## 🔍 How It Works

1. **Template Rendering:**
   - Jinja2 renders template with provided variables
   - `StrictUndefined` catches missing variables immediately

2. **Post-Render Validation:**
   - Regex scan for leftover `{{ VAR }}` patterns
   - Exclude Jinja2 literals and statements
   - Return list of unreplaced variables (if any)

3. **Error Reporting:**
   - Raise `TemplateValidationError` if issues found
   - Include helpful context (function, file, variables)
   - Fail fast before script is written to disk or sent to cloud

## ⚙️ Configuration

### Strict Mode (Default)

Enabled by default in production:

```python
# In generate_boot_scripts.py

def _get_jinja2_env(strict: bool = True):
    """Get Jinja2 environment with configurable strictness."""
    return Environment(
        loader=FileSystemLoader(script_dir),
        undefined=StrictUndefined if strict else Undefined,
        autoescape=False
    )
```

### Opt-Out for Testing

Allow disabling strict mode for unit tests:

```python
# In tests
def test_generate_shell_script_optional_vars():
    env = _get_jinja2_env(strict=False)
    # Allow missing variables in tests
```

## 🚨 Best Practices

### Variable Provisioning

**DO:**
```python
# Explicitly provide all variables
rendered = template.render(
    TAILSCALE_KEY=tailscale_key,
    LEADER_IP=leader_ip,
    ROLE=role,
    HAS_GPU="true" if has_gpu else "false"
)
```

**DON'T:**
```python
# Rely on defaults or optional behavior
rendered = template.render()  # Missing variables!
```

### Variable Naming

**DO:**
```python
# Clear, descriptive names
TAILSCALE_KEY
LEADER_IP
ENABLE_SPOT_HANDLING
```

**DON'T:**
```python
# Ambiguous or abbreviated names
TS_KEY
L_IP
SPOT_EN
```

### Default Values

**DO:**
```python
# Provide defaults in Python code
CUDA_VERSION=cuda_version or "12.1"
DRIVER_VERSION=driver_version or "535"
```

**DON'T:**
```python
# Rely on Jinja2 defaults
{{ CUDA_VERSION or "12.1" }}  # Harder to validate
```

## ⚠️ Limitations & Considerations

### False Positives

**Scenario:** Legitimate use of `{{` in output (e.g., heredoc, string escaping)

**Mitigation:**
- Escape with `{{ '{{' }}` in Jinja2 templates
- Use Jinja2 `{% raw %}...{% endraw %}` blocks for literal content
- Whitelist specific patterns in validation

**Example:**
```bash
# In boot.sh template
cat <<EOF > /tmp/config
{{ "{{" }}  # Renders as literal {{
SOME_VALUE={{ SOME_VALUE }}
{{ "}}" }}  # Renders as literal }}
EOF
```

### Performance

**Impact:** Minimal
- Regex scan is O(n) where n = template length
- Typical boot script ~100 lines, scans in <1ms
- One-time cost at provisioning time

### Backward Compatibility

**Breaking Changes:**
- `StrictUndefined` will break if existing code has undefined variables
- Tests may need updates to provide all variables

**Migration Strategy:**
1. Add validation as opt-in (`strict=False` by default)
2. Run with validation in dev/staging
3. Fix any missing variable issues
4. Enable strict mode by default

## 🔗 Related Features

- **boot_consul_nomad:** Boot script templates (this feature enhances)
- **provision_node:** Calls `generate_shell_script()` to create user data
- **test_boot.py:** Unit tests for boot script generation

## 📚 References

- [Jinja2 Undefined Behavior](https://jinja.palletsprojects.com/en/3.1.x/api/#undefined-types)
- [Jinja2 StrictUndefined](https://jinja.palletsprojects.com/en/3.1.x/api/#jinja2.StrictUndefined)
- [Python re Module](https://docs.python.org/3/library/re.html)

## 🎯 Success Criteria

- [ ] StrictUndefined mode enabled by default
- [ ] Post-render validation catches unreplaced variables
- [ ] Clear error messages with variable names
- [ ] Unit tests for validation logic
- [ ] Backward compatibility maintained (opt-out available)
- [ ] Documentation updated

## 📝 Implementation Checklist

- [x] Analyze current Jinja2 usage (already implemented)
- [ ] Create `TEMPLATE_VALIDATION.md` (this file)
- [ ] Add `TemplateValidationError` exception class
- [ ] Implement `validate_rendered_template()` function
- [ ] Update `_get_jinja2_env()` to support strict mode
- [ ] Add validation to `generate_shell_script()`
- [ ] Add validation to `generate_cloud_init_yaml()`
- [ ] Create unit tests for validation
- [ ] Update existing tests to provide all variables
- [ ] Update documentation

## 💡 Usage Examples

### Normal Usage (Automatic Validation)

```python
from src.infrastructure.boot_consul_nomad.generate_boot_scripts import generate_shell_script

# Validation happens automatically
script = generate_shell_script(
    tailscale_key="ts-key-123",
    leader_ip="10.0.0.1",
    role="server"
)
# Returns validated script, or raises TemplateValidationError
```

### With Missing Variable (Error)

```python
# Missing LEADER_IP variable
script = generate_shell_script(
    tailscale_key="ts-key-123",
    # leader_ip is MISSING
    role="server"
)
# Raises: TemplateValidationError
# Message: "Unreplaced variables detected: LEADER_IP"
```

### Opt-Out of Strict Mode (Testing)

```python
# In unit tests, allow missing variables
env = _get_jinja2_env(strict=False)
template = env.get_template("boot.sh")
rendered = template.render(TAILSCALE_KEY="test")  # LEADER_IP missing, but no error
```
