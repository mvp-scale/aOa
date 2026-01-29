# =============================================================================
# SECTION 65: Job Queue Commands (GL-089)
# =============================================================================
# Universal work queue for aOa background tasks.
# Queue is the single source of truth for all pending work.

cmd_jobs() {
    # aoa jobs - Job queue management
    # Usage: aoa jobs [SUBCOMMAND]
    #
    # Subcommands:
    #   status   - Show queue status (default)
    #   pending  - List pending jobs
    #   process  - Process N jobs
    #   retry    - Retry failed jobs
    #   clear    - Clear completed/failed jobs

    local project_id=$(get_project_id)

    case "${1:-status}" in
        status)
            # Show queue status
            local result=$(curl -sf "${INDEX_URL}/jobs/status?project_id=${project_id}" 2>/dev/null)
            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not fetch job status${NC}" >&2
                return 1
            fi

            local pending=$(echo "$result" | jq -r '.pending // 0')
            local active=$(echo "$result" | jq -r '.active // 0')
            local complete=$(echo "$result" | jq -r '.complete // 0')
            local failed=$(echo "$result" | jq -r '.failed // 0')

            echo -e "${CYAN}${BOLD}⚡ aOa Jobs${NC}  ${pending} pending ${DIM}│${NC} ${active} active ${DIM}│${NC} ${GREEN}${complete} complete${NC} ${DIM}│${NC} ${RED}${failed} failed${NC}"

            # Show hints based on state
            if [ "$failed" -gt 0 ]; then
                echo -e "${DIM}Run 'aoa jobs failed' to see errors, 'aoa jobs retry' to retry${NC}"
            elif [ "$pending" -gt 0 ] && [ "$active" -eq 0 ]; then
                echo -e "${DIM}Run 'aoa jobs process' to process pending jobs${NC}"
            fi
            ;;

        pending)
            # List pending jobs
            shift
            local limit="${1:-10}"
            local result=$(curl -sf "${INDEX_URL}/jobs/pending?project_id=${project_id}&limit=${limit}" 2>/dev/null)
            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not fetch pending jobs${NC}" >&2
                return 1
            fi

            local count=$(echo "$result" | jq -r '.count // 0')
            if [ "$count" -eq 0 ]; then
                echo "No pending jobs"
                return 0
            fi

            echo -e "${CYAN}${BOLD}Pending Jobs${NC} (${count})"
            echo "$result" | jq -r '.jobs[] | "  \(.type): \(.payload.domain // .payload.files // "-")"'
            ;;

        process)
            # Process jobs
            shift
            local count="${1:-3}"
            local result=$(curl -sf -X POST "${INDEX_URL}/jobs/process" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"count\":${count}}" 2>/dev/null)

            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not process jobs${NC}" >&2
                return 1
            fi

            local processed=$(echo "$result" | jq -r '.processed // 0')
            echo "Processed ${processed} jobs"

            # Show updated status
            local pending=$(echo "$result" | jq -r '.status.pending // 0')
            local failed=$(echo "$result" | jq -r '.status.failed // 0')
            if [ "$pending" -gt 0 ]; then
                echo "${pending} jobs remaining"
            fi
            if [ "$failed" -gt 0 ]; then
                echo -e "${RED}${failed} jobs failed${NC}"
            fi
            ;;

        failed)
            # List failed jobs with errors
            shift
            local limit="${1:-10}"
            local result=$(curl -sf "${INDEX_URL}/jobs/failed?project_id=${project_id}&limit=${limit}" 2>/dev/null)
            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not fetch failed jobs${NC}" >&2
                return 1
            fi

            local count=$(echo "$result" | jq -r '.count // 0')
            if [ "$count" -eq 0 ]; then
                echo "No failed jobs"
                return 0
            fi

            echo -e "${RED}${BOLD}Failed Jobs${NC} (${count})"
            echo "$result" | jq -r '.jobs[] | "  \(.payload.domain // .payload.files // "-"): \(.error // "unknown error")"'
            echo ""
            echo -e "${DIM}Run 'aoa jobs retry' to move back to pending${NC}"
            ;;

        retry)
            # Retry failed jobs
            local result=$(curl -sf -X POST "${INDEX_URL}/jobs/retry" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\"}" 2>/dev/null)

            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not retry jobs${NC}" >&2
                return 1
            fi

            local retried=$(echo "$result" | jq -r '.retried // 0')
            echo "Retried ${retried} jobs"
            ;;

        enrich)
            # Queue domains for enrichment
            shift
            local file="${1:-.aoa/domains/intelligence.json}"

            if [ ! -f "$file" ]; then
                echo -e "${RED}Error: File not found: ${file}${NC}" >&2
                return 1
            fi

            local domains=$(cat "$file")
            local result=$(curl -sf -X POST "${INDEX_URL}/jobs/push/enrich" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"domains\":${domains}}" 2>/dev/null)

            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not queue jobs${NC}" >&2
                return 1
            fi

            local queued=$(echo "$result" | jq -r '.queued // 0')
            echo -e "${GREEN}✓${NC} Queued ${BOLD}${queued}${NC} domains for enrichment"
            ;;

        clear)
            # Clear queues
            shift
            local queue="${1:-complete}"
            local result=$(curl -sf -X POST "${INDEX_URL}/jobs/clear" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"queue\":\"${queue}\"}" 2>/dev/null)

            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not clear jobs${NC}" >&2
                return 1
            fi

            echo "Cleared: $(echo "$result" | jq -c '.')"
            ;;

        --help|-h)
            echo "Usage: aoa jobs [SUBCOMMAND]"
            echo ""
            echo "Job queue management."
            echo ""
            echo "Subcommands:"
            echo "  status          Show queue status (default)"
            echo "  pending [N]     List N pending jobs (default: 10)"
            echo "  failed [N]      List N failed jobs with errors (default: 10)"
            echo "  process [N]     Process N jobs (default: 3)"
            echo "  enrich [FILE]   Queue domains for enrichment"
            echo "  retry           Retry all failed jobs"
            echo "  clear [TYPE]    Clear queue (complete|failed|all)"
            echo ""
            echo "Examples:"
            echo "  aoa jobs                 # Show status"
            echo "  aoa jobs pending 5       # Show 5 pending jobs"
            echo "  aoa jobs process 3       # Process 3 jobs"
            echo "  aoa jobs enrich          # Queue from intelligence.json"
            echo "  aoa jobs retry           # Retry failed"
            echo "  aoa jobs clear complete  # Clear completed jobs"
            ;;

        *)
            echo -e "${RED}Unknown subcommand: $1${NC}" >&2
            echo "Run 'aoa jobs --help' for usage" >&2
            return 1
            ;;
    esac
}
