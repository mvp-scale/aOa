# =============================================================================
# SECTION 90: Main Entry Point
# =============================================================================
#
# PURPOSE
#   Command-line argument parsing and routing. This is where execution starts.
#   Maps user commands to the appropriate cmd_* functions.
#
# DEPENDENCIES
#   - All previous sections must be loaded
#
# ROUTING
#   Arguments are matched against command names and dispatched to handlers.
#   Unknown commands show help text.
#
# =============================================================================

# =============================================================================
# Main
# =============================================================================

main() {
    local cmd="${1:-help}"
    shift || true

    case "$cmd" in
        # Project Management (new in v2)
        init)       cmd_init "$@" ;;
        remove|rm)  cmd_remove "$@" ;;
        projects)   cmd_projects "$@" ;;

        # Session
        history|h)  cmd_history "$@" ;;
        reset)      cmd_reset "$@" ;;
        wipe)       cmd_wipe "$@" ;;

        # Search (Unix parity)
        grep|g)     cmd_grep "$@" ;;
        egrep|eg)   cmd_egrep "$@" ;;

        # Search (deprecated aliases)
        search|s)   cmd_grep "$@" ;;
        multi|m)    cmd_grep -a "$@" ;;
        pattern|p)  cmd_egrep "$@" ;;

        # Local Index
        changes|c)  cmd_changes "$@" ;;
        files|f)    cmd_files "$@" ;;
        outline|o)  cmd_outline "$@" ;;
        enrich)     cmd_enrich "$@" ;;

        # File Discovery (Unix parity)
        find)       cmd_find "$@" ;;
        tree)       cmd_tree "$@" ;;
        locate)     cmd_locate "$@" ;;
        head)       cmd_head "$@" ;;
        tail)       cmd_tail "$@" ;;
        lines)      cmd_lines "$@" ;;

        # Behavioral (aOa unique)
        hot)        cmd_hot "$@" ;;
        touched)    cmd_touched "$@" ;;
        focus)      cmd_focus "$@" ;;
        predict)    cmd_predict "$@" ;;

        # Intent Tracking
        intent|i)   cmd_intent "$@" ;;

        # Whitelist Management
        whitelist|w) cmd_whitelist "$@" ;;

        # Knowledge Repos
        repo|r)     cmd_repo "$@" ;;

        # System
        start)      cmd_start ;;
        stop)       cmd_stop ;;
        env)        cmd_env ;;
        port)       cmd_port "$@" ;;
        quickstart|qs) cmd_quickstart "$@" ;;
        analyze|az) cmd_analyze "$@" ;;
        learn)      cmd_learn "$@" ;;
        domains|d)  cmd_domains "$@" ;;
        stats)      cmd_stats ;;
        health)     cmd_health ;;
        metrics)    cmd_metrics ;;
        baseline|bl) cmd_baseline ;;
        memory|mem) cmd_memory "$@" ;;
        services|svc|map) cmd_services ;;
        info)       cmd_info ;;
        help|--help|-h) cmd_help ;;

        *)
            echo -e "${RED}Unknown command: $cmd${NC}"
            echo "Run 'aoa help' for usage"
            return 1
            ;;
    esac
}

main "$@"
