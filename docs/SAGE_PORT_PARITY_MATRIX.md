# DevTrack Sage reference parity matrix

Pinned Python baseline: `D:\git_apps\ai_sessions_skills` commit `94a2544f8c85a630fa8b5d9a94d9938121aef11b`. This is an inventory and assignment, not a claim that later Go behavior is implemented. All 135 `test_*` methods in the two pinned regression files are listed below. Go-only v1 contract coverage is in `internal/sage/event_test.go`.

Supplemental compatibility reference: commit `b85a1ab`. Its configurable Codex `cli` history
source and Windows no-popup installer behavior are tracked separately and do not change the
135-scenario baseline. `TestNormalizeCodexHistoryItemSupportsIDEAndCLI` covers the pure Go
normalization seam; SQLite polling and installer selection remain SAGE-002 work.

`Excluded` means the Python behavior conflicts with the local SQLite, no-implicit-Git-write Sage contract; the exclusion is deliberate and must be revisited if that product contract changes. Python-specific watcher subprocess mechanics are assigned to SAGE-002 as daemon-lifecycle behavior rather than copied literally.

| Python regression scenario | Go destination | Assignment | Port note |
|---|---|---|---|
| `tool/tests/test_ide_capture.py:55` `test_captures_original_command_once_and_excludes_cli_and_old_history` | `internal/sage/hooks/ide` | SAGE-004 | Port as an optional, versioned IDE adapter |
| `tool/tests/test_ide_capture.py:66` `test_in_progress_command_is_collected_when_it_finishes` | `internal/sage/hooks/ide` | SAGE-004 | Port as an optional, versioned IDE adapter |
| `tool/tests/test_ide_capture.py:78` `test_pause_does_not_read_or_advance_history` | `internal/sage/hooks/ide` | SAGE-004 | Port as an optional, versioned IDE adapter |
| `tool/tests/test_ide_capture.py:86` `test_reset_excludes_commands_from_the_paused_period` | `internal/sage/hooks/ide` | SAGE-004 | Port as an optional, versioned IDE adapter |
| `tool/tests/test_ide_capture.py:92` `test_failure_does_not_advance_checkpoint` | `internal/sage/hooks/ide` | SAGE-004 | Port as an optional, versioned IDE adapter |
| `tool/tests/test_ide_capture.py:100` `test_failed_command_preserves_exit_status` | `internal/sage/hooks/ide` | SAGE-004 | Port as an optional, versioned IDE adapter |
| `tool/tests/test_portable.py:45` `test_windows_uses_appdata` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:50` `test_windows_falls_back_to_home` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:54` `test_linux_honours_xdg` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:59` `test_linux_default_is_dot_config` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:63` `test_macos_default_is_dot_config` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:75` `test_linux_devin_under_xdg` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:81` `test_linux_respects_xdg_override` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:85` `test_windows_devin_under_appdata` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:91` `test_dot_dirs_are_home_relative_on_every_platform` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:101` `test_windows_prefers_existing_opencode_dir` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:107` `test_every_harness_has_a_skill_destination` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:115` `test_launcher_extension_matches_platform` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:122` `test_both_platform_shims_are_shipped` | `internal/sage/config + hooks` | SAGE-002 | Port platform root and installer behavior |
| `tool/tests/test_portable.py:129` `test_placeholders_are_rendered_and_empty_model_dropped` | `internal/sage/distill` | SAGE-003 | Port backend selection without Python |
| `tool/tests/test_portable.py:148` `test_no_backend_when_binary_missing` | `internal/sage/distill` | SAGE-003 | Port backend selection without Python |
| `tool/tests/test_portable.py:158` `test_shipped_backends_declare_a_prompt_mode` | `internal/sage/distill` | SAGE-003 | Port backend selection without Python |
| `tool/tests/test_portable.py:171` `test_same_action_different_values_collapses` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:177` `test_names_do_not_leak_into_signatures` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:184` `test_real_subcommands_still_split` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:190` `test_distinct_actions_stay_distinct` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:195` `test_pipelines_yield_one_signature_per_segment` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:199` `test_quoted_separators_do_not_split` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:202` `test_trivial_commands_are_ignored` | `internal/sage/knowledge` | SAGE-003 | Port command signatures |
| `tool/tests/test_portable.py:230` `test_every_shape_produces_a_shell_record` | `internal/sage/capture` | SAGE-002 | Port harness payload normalization |
| `tool/tests/test_portable.py:238` `test_string_encoded_arguments_are_parsed` | `internal/sage/capture` | SAGE-002 | Port harness payload normalization |
| `tool/tests/test_portable.py:244` `test_failure_shapes_are_detected` | `internal/sage/capture` | SAGE-002 | Port harness payload normalization |
| `tool/tests/test_portable.py:257` `test_search_tools_become_ripgrep` | `internal/sage/capture` | SAGE-002 | Port harness payload normalization |
| `tool/tests/test_portable.py:271` `test_edit_tools_are_documented_once_per_kind` | `internal/sage/capture` | SAGE-002 | Port harness payload normalization |
| `tool/tests/test_portable.py:279` `test_unknown_payload_is_ignored_not_crashed` | `internal/sage/capture` | SAGE-002 | Port harness payload normalization |
| `tool/tests/test_portable.py:286` `test_windows_paths_do_not_duplicate_hooks_on_reinstall` | `internal/sage/hooks` | SAGE-002 | Port additive install/removal |
| `tool/tests/test_portable.py:300` `test_merge_preserves_foreign_settings_and_is_idempotent` | `internal/sage/hooks` | SAGE-002 | Port additive install/removal |
| `tool/tests/test_portable.py:319` `test_remove_is_pure_subtraction` | `internal/sage/hooks` | SAGE-002 | Port additive install/removal |
| `tool/tests/test_portable.py:328` `test_legacy_marker_purges_old_layout` | `internal/sage/hooks` | SAGE-002 | Port additive install/removal |
| `tool/tests/test_portable.py:339` `test_missing_file_reports_missing_on_remove` | `internal/sage/hooks` | SAGE-002 | Port additive install/removal |
| `tool/tests/test_portable.py:344` `test_lock_is_exclusive_and_releasable` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:354` `test_stale_lock_is_reclaimed` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:363` `test_windows_helpers_request_no_console` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:368` `test_posix_helpers_do_not_receive_windows_flags` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:374` `test_real_helper_has_no_console_and_preserves_piped_input` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:382` `test_detached_process_has_no_console` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:397` `test_tasklist_access_failure_does_not_report_a_dead_process` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:402` `test_dead_pid_is_not_alive` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:408` `test_missing_pidfile_is_not_alive` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:412` `test_own_pid_with_fresh_touch_is_alive` | `internal/sage/importer` | SAGE-002 | Adapt to daemon-owned lifecycle |
| `tool/tests/test_portable.py:457` `test_gitignored_runtime_is_never_dirty` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:461` `test_nested_knowledge_folder_commits_with_root_relative_paths` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:471` `test_preexisting_work_is_left_uncommitted` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:479` `test_entangled_file_is_skipped_not_guessed` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:489` `test_user_staged_work_stays_staged` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:499` `test_paths_with_spaces_survive` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:505` `test_deletion_by_writer_is_committed` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:512` `test_nothing_of_ours_makes_no_commit` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:518` `test_auto_commit_off_is_respected` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:526` `test_non_repo_is_a_no_op` | `none` | Excluded | Python writer's implicit Git auto-commit is intentionally not a Sage behavior; durable storage is local SQLite |
| `tool/tests/test_portable.py:547` `test_an_existing_entry_routes_its_whole_binary` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:553` `test_a_binary_the_list_never_knew_still_routes` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:559` `test_unknown_binary_has_no_route_yet` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:562` `test_remembered_decisions_survive_before_any_entry_exists` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:566` `test_entries_win_over_a_stale_remembered_route` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:571` `test_title_is_read_back_from_the_file` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:575` `test_skipped_and_readme_never_become_routes` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:579` `test_filename_cannot_escape_the_knowledge_base` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:590` `test_fallback_is_a_file_of_its_own` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:593` `test_merge_moves_entries_and_repoints_routing` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:605` `test_merge_is_a_no_op_for_a_missing_file` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:608` `test_a_correction_outlives_the_classifier` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:615` `test_parses_heading_sigs_and_commands` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:623` `test_multiple_sigs_on_one_entry` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:628` `test_action_is_derived_from_the_command_block` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:631` `test_same_action_different_flags_is_a_match` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:636` `test_different_subcommand_is_not_a_match` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:649` `test_add_sig_appends_and_is_idempotent` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:658` `test_insert_creates_file_with_header` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:665` `test_insert_reuses_an_existing_section` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:674` `test_insert_preserves_earlier_entries` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:681` `test_skipped_file_records_the_sig` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:685` `test_documented_sigs_spans_every_file` | `internal/sage/knowledge` | SAGE-003 | Port routing, merge, and rendering |
| `tool/tests/test_portable.py:703` `test_clean_response` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:708` `test_answer_wrapped_in_a_fence_or_chatter` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:716` `test_model_emitted_sig_is_stripped` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:723` `test_skip_verdict_is_distinguished_from_malformed` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:735` `test_empty_commands_fall_back_to_the_captured_command` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:741` `test_garbage_is_rejected_not_written` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:759` `test_renders_a_parseable_entry` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:766` `test_blank_notes_omits_the_bullet` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:770` `test_multiline_prose_is_flattened` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:777` `test_exact_and_case_insensitive_match_reuse` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:780` `test_near_duplicate_snaps_to_existing` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:785` `test_genuinely_new_section_is_kept` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:788` `test_related_but_differently_worded_sections_are_not_merged` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:799` `test_empty_falls_back_to_default` | `internal/sage/distill + knowledge` | SAGE-003 | Port validated model output and deterministic fallback |
| `tool/tests/test_portable.py:824` `test_pause_is_persistent_and_reversible` | `internal/sage/state_test.go` | SAGE-001 | `TestPauseResumeStateIsPersistentAndIdempotent` |
| `tool/tests/test_portable.py:832` `test_resume_when_not_paused_is_harmless` | `internal/sage/state_test.go` | SAGE-001 | `TestPauseResumeStateIsPersistentAndIdempotent` |
| `tool/tests/test_portable.py:836` `test_env_switch_is_process_local` | `internal/sage/config` | SAGE-002 | Adapt to Go client configuration; no Python process switch |
| `tool/tests/test_portable.py:842` `test_paused_stops_capture` | `internal/sage/capture` | SAGE-002 | Prove against an installed adapter |
| `tool/tests/test_portable.py:846` `test_log_records_a_decision_per_line` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:855` `test_log_flattens_newlines_so_one_action_is_one_line` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:859` `test_log_never_raises` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:864` `test_enqueue_logs_new_then_known` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:873` `test_new_knowledge_base_never_tracks_the_queue` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:881` `test_existing_gitignore_is_extended_not_replaced` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:892` `test_repeat_within_one_command_line_is_marked` | `internal/sage/capture + logging` | SAGE-002 | Port silent capture and sanitized diagnostics |
| `tool/tests/test_portable.py:919` `test_stop_and_start_round_trip` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:925` `test_restart_replaces_the_watcher_and_stop_releases_it` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:938` `test_aliases_all_work` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:948` `test_a_typo_is_loud_rather_than_a_silent_no_op` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:954` `test_stop_records_its_reason` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:959` `test_hook_verbs_are_silent_and_never_fail` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:965` `test_a_legacy_shim_cannot_silently_restart_capture` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:971` `test_status_reports_both_states` | `internal/sage/importer + CLI` | SAGE-002 | Adapt watcher to DevTrack daemon |
| `tool/tests/test_portable.py:1008` `test_normal_reply_is_returned` | `internal/sage/distill` | SAGE-003 | Port model failure semantics |
| `tool/tests/test_portable.py:1011` `test_token_limit_with_no_output_is_a_failure_not_a_verdict` | `internal/sage/distill` | SAGE-003 | Port model failure semantics |
| `tool/tests/test_portable.py:1017` `test_shipped_ollama_backend_has_no_output_cap` | `internal/sage/distill` | SAGE-003 | Port model failure semantics |
| `tool/tests/test_portable.py:1023` `test_text_backend_defers_prompt_file_substitution` | `internal/sage/distill` | SAGE-003 | Port model failure semantics |
| `tool/tests/test_portable.py:1074` `test_documents_and_routes` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1081` `test_second_variant_merges_without_calling_the_model` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1093` `test_already_documented_sig_is_untouched` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1101` `test_model_outage_is_not_recorded_as_skipped` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1111` `test_skip_verdict_is_recorded_so_it_never_returns` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1119` `test_malformed_reply_is_retried_under_the_schema` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1136` `test_skip_verdict_is_not_retried` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1149` `test_unknown_binary_is_classified_once_then_remembered` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1174` `test_classifier_choice_is_sanitised` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1190` `test_unreachable_classifier_falls_back_to_its_own_file` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1202` `test_existing_sections_are_offered_to_the_model` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1215` `test_agent_prompt_uses_separate_runtime_folder` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1225` `test_runtime_setting_is_relative_to_config` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1230` `test_explicit_kb_override_keeps_runtime_isolated` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
| `tool/tests/test_portable.py:1234` `test_keyword_search_matches_whole_entries_and_filters_topics` | `internal/sage/knowledge + distill` | SAGE-003 | Port knowledge processing and search |
