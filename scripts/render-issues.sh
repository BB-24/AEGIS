#!/usr/bin/env bash
# render-issues.sh
# Extracts and renders issues from the AEGIS implementation plan
# Supports multiple output formats: text (default), json, markdown, csv

set -euo pipefail

# ==============================================================================
# Configuration
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMPL_PLAN="${PROJECT_ROOT}/docs/implementation-plan.md"

# Default output format
FORMAT="text"

# Colors for text output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ==============================================================================
# Helper functions
# ==============================================================================

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Extract and render issues from the AEGIS implementation plan.

OPTIONS:
    -f, --format FORMAT    Output format: text, json, markdown, csv (default: text)
    -h, --help             Show this help message
    -t, --task TASK_NUM    Filter by specific task number (e.g., 1, 2, 3...)
    -s, --status STATUS    Filter by status: pending, in-progress, done (default: all)
    --phase PHASE          Filter by phase number (1-16)
    -o, --output FILE      Write output to file instead of stdout

EXAMPLES:
    $(basename "$0")                    # Render all issues as text
    $(basename "$0") -f json            # Render as JSON
    $(basename "$0") -f md -o issues.md # Render as Markdown to file
    $(basename "$0") -t 5               # Show only Task 5 issues
    $(basename "$0") --phase 3          # Show only Phase 3 issues

EOF
}

log() {
    echo -e "${BLUE}[INFO]${NC} $*" >&2
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
    exit 1
}

# ==============================================================================
# Parse implementation plan
# ==============================================================================

parse_implementation_plan() {
    if [[ ! -f "${IMPL_PLAN}" ]]; then
        error "Implementation plan not found at ${IMPL_PLAN}"
    fi

    # Extract tasks using awk - look for "### 7.X" section headers and "Task N — Title" patterns
    awk '
    BEGIN {
        task_num = 0
        in_task = 0
        in_subtasks = 0
        in_dod = 0
        phase = "Implementation"
        section = ""
        old_section = ""
        task_title = ""
        subtasks = ""
        dod = ""
        printed = 0
    }

    # Match phase headers (## X. Title) - e.g., "## 7. Implementation phases"
    /^## [0-9]+\. Implementation/ {
        phase = $0
        gsub(/^## +/, "", phase)
    }

    # Match section headers (### 7.X SectionName)
    /^### 7\.[0-9]+ / {
        if (in_task && task_num > 0 && !printed) {
            print_task()
            printed = 1
        }
        # Save current section for the task being printed
        old_section = section
        section = $0
        gsub(/^### +/, "", section)
        in_task = 0
        in_subtasks = 0
        in_dod = 0
        subtasks = ""
        dod = ""
        printed = 0
    }

    # Match task headers (Task N — Title)
    /^Task [0-9]+ [—-]/ {
        if (in_task && task_num > 0 && !printed) {
            print_task()
            printed = 1
        }
        in_task = 1
        in_subtasks = 0
        in_dod = 0
        subtasks = ""
        dod = ""
        printed = 0
        
        # Extract task number and title using substr/index instead of match()
        # Format: "Task N — Title" or "Task NN — Title"
        dash_pos = index($0, "—")
        if (dash_pos == 0) dash_pos = index($0, "-")
        if (dash_pos > 0) {
            # Find task number: after "Task " and before " —"
            task_start = index($0, "Task ") + 5
            task_end = dash_pos - 2
            if (task_end >= task_start) {
                task_num = substr($0, task_start, task_end - task_start + 1)
                gsub(/^[ \t]+|[ \t]+$/, "", task_num)
            }
            # Title is after "— "
            task_title = substr($0, dash_pos + 2)
            gsub(/^[ \t]+|[ \t]+$/, "", task_title)
        }
    }

    # Match subtasks section
    /^Subtasks:/ {
        in_subtasks = 1
        in_dod = 0
        next
    }

    # Match Definition of done section
    /^Definition of done:/ {
        in_subtasks = 0
        in_dod = 1
        next
    }

    # Capture subtask items
    in_subtasks && /^[0-9]+\. / {
        gsub(/^[ \t]+|[ \t]+$/, "", $0)
        if (subtasks != "") subtasks = subtasks "|"
        subtasks = subtasks $0
        next
    }

    # Capture DoD items
    in_dod && /^- / {
        gsub(/^[ \t]+|[ \t]+$/, "", $0)
        gsub(/^- /, "", $0)
        if (dod != "") dod = dod "|"
        dod = dod $0
        next
    }

    # End of task (next section, or next phase)
    /^### 7\.[0-9]+ / || /^## [0-9]+\. Implementation/ {
        if (in_task && task_num > 0 && !printed) {
            print_task()
            printed = 1
        }
    }

    END {
        if (in_task && task_num > 0 && !printed) {
            print_task()
        }
    }

    function print_task() {
        full_title = task_title
        if (old_section != "") {
            full_title = "[" old_section "] " task_title
        }
        print task_num "|" phase "|" full_title "|" subtasks "|" dod
    }
    ' "${IMPL_PLAN}"
}

# ==============================================================================
# Output formatters
# ==============================================================================

render_text() {
    local task_num="$1"
    local phase="$2"
    local title="$3"
    local subtasks="$4"
    local dod="$5"
    
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}${CYAN}  Task ${task_num}: ${title}${NC}"
    echo -e "${BOLD}${CYAN}══════════════════════════════════════════════════════════════${NC}"
    echo -e "  ${YELLOW}Phase:${NC} ${phase}"
    echo ""
    
    if [[ -n "${subtasks}" ]]; then
        echo -e "  ${BOLD}Subtasks:${NC}"
        IFS='|' read -ra SUBTASK_ARRAY <<< "${subtasks}"
        for i in "${!SUBTASK_ARRAY[@]}"; do
            echo -e "    ${GREEN}☐${NC} ${SUBTASK_ARRAY[i]}"
        done
        echo ""
    fi
    
    if [[ -n "${dod}" ]]; then
        echo -e "  ${BOLD}Definition of Done:${NC}"
        IFS='|' read -ra DOD_ARRAY <<< "${dod}"
        for item in "${DOD_ARRAY[@]}"; do
            echo -e "    ${BLUE}✓${NC} ${item}"
        done
        echo ""
    fi
}

render_json() {
    local task_num="$1"
    local phase="$2"
    local title="$3"
    local subtasks="$4"
    local dod="$5"
    
    # Build subtasks array
    local subtasks_json="[]"
    if [[ -n "${subtasks}" ]]; then
        IFS='|' read -ra SUBTASK_ARRAY <<< "${subtasks}"
        subtasks_json="["
        for i in "${!SUBTASK_ARRAY[@]}"; do
            if [[ $i -gt 0 ]]; then
                subtasks_json="${subtasks_json},"
            fi
            subtasks_json="${subtasks_json}\"${SUBTASK_ARRAY[i]}\""
        done
        subtasks_json="${subtasks_json}]"
    fi
    
    # Build dod array
    local dod_json="[]"
    if [[ -n "${dod}" ]]; then
        IFS='|' read -ra DOD_ARRAY <<< "${dod}"
        dod_json="["
        for i in "${!DOD_ARRAY[@]}"; do
            if [[ $i -gt 0 ]]; then
                dod_json="${dod_json},"
            fi
            dod_json="${dod_json}\"${DOD_ARRAY[i]}\""
        done
        dod_json="${dod_json}]"
    fi
    
    cat <<EOF
  {
    "task": ${task_num},
    "phase": "${phase}",
    "title": "${title}",
    "subtasks": ${subtasks_json},
    "definition_of_done": ${dod_json}
  }
EOF
}

render_markdown() {
    local task_num="$1"
    local phase="$2"
    local title="$3"
    local subtasks="$4"
    local dod="$5"
    
    echo "## Task ${task_num}: ${title}"
    echo ""
    echo "**Phase:** ${phase}"
    echo ""
    
    if [[ -n "${subtasks}" ]]; then
        echo "### Subtasks"
        echo ""
        IFS='|' read -ra SUBTASK_ARRAY <<< "${subtasks}"
        for item in "${SUBTASK_ARRAY[@]}"; do
            echo "- [ ] ${item}"
        done
        echo ""
    fi
    
    if [[ -n "${dod}" ]]; then
        echo "### Definition of Done"
        echo ""
        IFS='|' read -ra DOD_ARRAY <<< "${dod}"
        for item in "${DOD_ARRAY[@]}"; do
            echo "- [ ] ${item}"
        done
        echo ""
    fi
    
    echo "---"
    echo ""
}

render_csv() {
    local task_num="$1"
    local phase="$2"
    local title="$3"
    local subtasks="$4"
    local dod="$5"
    
    # Escape quotes and commas for CSV
    local esc_phase="${phase//\"/\"\"}"
    local esc_title="${title//\"/\"\"}"
    local esc_subtasks="${subtasks//|/; }"
    esc_subtasks="${esc_subtasks//\"/\"\"}"
    local esc_dod="${dod//|/; }"
    esc_dod="${esc_dod//\"/\"\"}"
    
    echo "${task_num},\"${esc_phase}\",\"${esc_title}\",\"${esc_subtasks}\",\"${esc_dod}\""
}

# ==============================================================================
# Main processing
# ==============================================================================

main() {
    local filter_task=""
    local filter_status=""
    local filter_phase=""
    local output_file=""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -f|--format)
                FORMAT="$2"
                shift 2
                ;;
            -t|--task)
                filter_task="$2"
                shift 2
                ;;
            -s|--status)
                filter_status="$2"
                shift 2
                ;;
            --phase)
                filter_phase="$2"
                shift 2
                ;;
            -o|--output)
                output_file="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done
    
    # Validate format
    case "${FORMAT}" in
        text|json|markdown|md|csv) ;;
        *) error "Invalid format: ${FORMAT}. Use: text, json, markdown, csv" ;;
    esac
    
    # Normalize format
    [[ "${FORMAT}" == "md" ]] && FORMAT="markdown"
    
    # Set up output
    local output_cmd="cat"
    if [[ -n "${output_file}" ]]; then
        output_cmd="tee ${output_file}"
    fi
    
    # Print header for JSON
    if [[ "${FORMAT}" == "json" ]]; then
        echo "[" | ${output_cmd}
    fi
    
    # Print header for CSV
    if [[ "${FORMAT}" == "csv" ]]; then
        echo "Task,Phase,Title,Subtasks,Definition of Done" | ${output_cmd}
    fi
    
    local first=true
    local count=0
    
    # Process each task
    while IFS='|' read -r task_num phase title subtasks dod; do
        # Skip empty lines
        [[ -z "${task_num}" ]] && continue
        
        # Apply filters
        if [[ -n "${filter_task}" && "${task_num}" != "${filter_task}" ]]; then
            continue
        fi
        
        if [[ -n "${filter_phase}" ]]; then
            if [[ "${phase}" != *"${filter_phase}"* ]]; then
                continue
            fi
        fi
        
        # Render based on format
        case "${FORMAT}" in
            text)
                render_text "${task_num}" "${phase}" "${title}" "${subtasks}" "${dod}" | ${output_cmd}
                ;;
            json)
                if [[ "${first}" == "true" ]]; then
                    first=false
                else
                    echo "," | ${output_cmd}
                fi
                render_json "${task_num}" "${phase}" "${title}" "${subtasks}" "${dod}" | ${output_cmd}
                ;;
            markdown)
                render_markdown "${task_num}" "${phase}" "${title}" "${subtasks}" "${dod}" | ${output_cmd}
                ;;
            csv)
                render_csv "${task_num}" "${phase}" "${title}" "${subtasks}" "${dod}" | ${output_cmd}
                ;;
        esac
        
        count=$((count + 1))
    done < <(
        awk '
        BEGIN {
            task_num = 0
            in_task = 0
            in_subtasks = 0
            in_dod = 0
            phase = "Implementation"
            section = ""
            old_section = ""
            task_title = ""
            subtasks = ""
            dod = ""
            printed = 0
        }

        # Match phase headers (## X. Title) - e.g., "## 7. Implementation phases"
        /^## [0-9]+\. Implementation/ {
            phase = $0
            gsub(/^## +/, "", phase)
        }

        # Match section headers (### 7.X SectionName)
        /^### 7\.[0-9]+ / {
            if (in_task && task_num > 0 && !printed) {
                print_task()
                printed = 1
            }
            # Save current section for the task being printed
            old_section = section
            section = $0
            gsub(/^### +/, "", section)
            in_task = 0
            in_subtasks = 0
            in_dod = 0
            subtasks = ""
            dod = ""
            printed = 0
        }

        # Match task headers (Task N — Title)
        /^Task [0-9]+ [—-]/ {
            if (in_task && task_num > 0 && !printed) {
                print_task()
                printed = 1
            }
            in_task = 1
            in_subtasks = 0
            in_dod = 0
            subtasks = ""
            dod = ""
            printed = 0
            
            # Extract task number and title using substr/index instead of match()
            # Format: "Task N — Title" or "Task NN — Title"
            dash_pos = index($0, "—")
            if (dash_pos == 0) dash_pos = index($0, "-")
            if (dash_pos > 0) {
                # Find task number: after "Task " and before " —"
                task_start = index($0, "Task ") + 5
                task_end = dash_pos - 2
                if (task_end >= task_start) {
                    task_num = substr($0, task_start, task_end - task_start + 1)
                    gsub(/^[ \t]+|[ \t]+$/, "", task_num)
                }
                # Title is after "— "
                task_title = substr($0, dash_pos + 2)
                gsub(/^[ \t]+|[ \t]+$/, "", task_title)
            }
        }

        # Match subtasks section
        /^Subtasks:/ {
            in_subtasks = 1
            in_dod = 0
            next
        }

        # Match Definition of done section
        /^Definition of done:/ {
            in_subtasks = 0
            in_dod = 1
            next
        }

        # Capture subtask items
        in_subtasks && /^[0-9]+\. / {
            gsub(/^[ \t]+|[ \t]+$/, "", $0)
            if (subtasks != "") subtasks = subtasks "|"
            subtasks = subtasks $0
            next
        }

        # Capture DoD items
        in_dod && /^- / {
            gsub(/^[ \t]+|[ \t]+$/, "", $0)
            gsub(/^- /, "", $0)
            if (dod != "") dod = dod "|"
            dod = dod $0
            next
        }

        # End of task (next section, or next phase)
        /^### 7\.[0-9]+ / || /^## [0-9]+\. Implementation/ {
            if (in_task && task_num > 0 && !printed) {
                print_task()
                printed = 1
            }
        }

        END {
            if (in_task && task_num > 0 && !printed) {
                print_task()
            }
        }

        function print_task() {
            full_title = task_title
            if (old_section != "") {
                full_title = "[" old_section "] " task_title
            }
            print task_num "|" phase "|" full_title "|" subtasks "|" dod
        }
        ' "${IMPL_PLAN}"
    )
    
    # Print footer for JSON
    if [[ "${FORMAT}" == "json" ]]; then
        echo "]" | ${output_cmd}
    fi
    
    if [[ "${FORMAT}" == "text" ]]; then
        echo -e "${BOLD}${GREEN}Total tasks rendered: ${count}${NC}" >&2
    fi
}

main "$@"