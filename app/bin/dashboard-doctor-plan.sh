#!/usr/bin/env bash
# Dash-Go Doctor repair-plan helpers. Sourced only by bin/doctor.sh.
# The plan is intentionally line-oriented so it remains usable over SSH and in
# Dashboard Control's captured output. Entries use: class<TAB>id<TAB>title<TAB>why<TAB>effect<TAB>preserves.

DOCTOR_PLAN_FILE="${DOCTOR_PLAN_FILE:-}"
DOCTOR_PLAN_OWN_FILE=0

_doctor_plan_clean(){ printf '%s' "${1:-}" | tr '\t\r\n' '   ' | sed 's/[[:space:]][[:space:]]*/ /g; s/^ //; s/ $//'; }
doctor_plan_selectable_class(){ case "${1:-}" in safe|guided|admin) return 0;; *) return 1;; esac; }

doctor_plan_init(){
  [ -n "$DOCTOR_PLAN_FILE" ] && return 0
  DOCTOR_PLAN_FILE="$(mktemp "${TMPDIR:-/tmp}/dash-go-doctor-plan.XXXXXX")" || return 1
  DOCTOR_PLAN_OWN_FILE=1
  : > "$DOCTOR_PLAN_FILE"
}

doctor_plan_cleanup(){
  [ "${DOCTOR_PLAN_OWN_FILE:-0}" = 1 ] && [ -n "${DOCTOR_PLAN_FILE:-}" ] && rm -f "$DOCTOR_PLAN_FILE" 2>/dev/null || true
}

doctor_plan_add(){
  # class: safe (eligible for all-safe), guided (select only), admin (select
  # only and may request sudo), repair (installer), manual (not automatable).
  # The optional detail field is reserved for --details output, keeping normal
  # repair plans readable while retaining exact technical evidence.
  local class="$1" id="$2" title why effect preserves details
  title="$(_doctor_plan_clean "$3")"
  why="$(_doctor_plan_clean "$4")"
  effect="$(_doctor_plan_clean "$5")"
  preserves="$(_doctor_plan_clean "$6")"
  details="$(_doctor_plan_clean "${7:-}")"
  doctor_plan_init || return 1
  grep -Fq "${class}"$'\t'"${id}"$'\t' "$DOCTOR_PLAN_FILE" 2>/dev/null && return 0
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$class" "$id" "$title" "$why" "$effect" "$preserves" "$details" >> "$DOCTOR_PLAN_FILE"
}

doctor_plan_has(){ [ -n "${DOCTOR_PLAN_FILE:-}" ] && [ -s "$DOCTOR_PLAN_FILE" ]; }

doctor_plan_action_ids(){
  local classes="${1:-safe}"
  [ -f "${DOCTOR_PLAN_FILE:-}" ] || return 0
  awk -F '\t' -v classes=",$classes," 'index(classes, "," $1 ",") { printf "%s%s", sep, $2; sep="," }' "$DOCTOR_PLAN_FILE"
}

doctor_plan_selectable_numbers(){
  # Keep hint numbering in the exact class-grouped order shown by render().
  local idx=0 class id title out="" wanted_class
  [ -f "${DOCTOR_PLAN_FILE:-}" ] || return 0
  for wanted_class in safe guided admin; do
    while IFS=$'\t' read -r class id title _; do
      [ "$class" = "$wanted_class" ] || continue
      idx=$((idx+1)); out="${out}${out:+,}[${idx}] ${title}"
    done < "$DOCTOR_PLAN_FILE"
  done
  printf '%s\n' "$out"
}

doctor_plan_render(){
  local detailed="${1:-0}" class heading idx=0 marker line title why effect preserves details id
  printf '\n== Repair plan\n'
  doctor_plan_has || { printf 'INFO No automatic repairs are currently planned.\n'; return 0; }
  printf 'INFO Review this plan before changing the device. Safe repairs are reversible where possible and never replace packaged application code.\n'
  for class in safe guided admin repair manual; do
    case "$class" in
      safe) heading="Safe repairs available — apply all is allowed" ;;
      guided) heading="Guided repairs — choose explicitly" ;;
      admin) heading="Administrator repairs — choose explicitly; may ask for sudo" ;;
      repair) heading="Installer repair required — Doctor will not replace packaged code" ;;
      manual) heading="Manual review needed — Doctor will not disrupt a live session" ;;
    esac
    grep -q "^${class}"$'\t' "$DOCTOR_PLAN_FILE" 2>/dev/null || continue
    printf '\n%s\n' "$heading"
    while IFS=$'\t' read -r _class id title why effect preserves details; do
      [ "$_class" = "$class" ] || continue
      if doctor_plan_selectable_class "$class"; then
        idx=$((idx+1)); marker="[$idx]"
      else
        marker="[info]"
      fi
      printf '  %s %s\n' "$marker" "$title"
      printf '      Why: %s\n' "$why"
      printf '      Result: %s\n' "$effect"
      [ -n "$preserves" ] && printf '      Preserves: %s\n' "$preserves"
      if [ "$class" = repair ]; then
        printf '      Next: run ~/install.sh --repair\n'
      elif [ "$class" = manual ]; then
        printf '      Next: manual review is required; Doctor will not auto-apply this item.\n'
      fi
      if [ "$detailed" = 1 ]; then
        printf '      Repair key: %s (%s)\n' "$id" "$class"
        [ -n "$details" ] && printf '      Details: %s\n' "$details"
      fi
    done < "$DOCTOR_PLAN_FILE"
  done
}

doctor_plan_ids_from_numbers(){
  # Map numbers in the same safe→guided→admin order as the renderer. Walking
  # discovery order here can run a different repair than the item displayed.
  local raw="$1" wanted="" idx=0 class id wanted_class
  raw="$(printf '%s' "$raw" | tr ', ' '\n' | awk 'NF && $0 ~ /^[0-9]+$/ {print}' | sort -n -u | tr '\n' ' ')"
  [ -n "$raw" ] || return 1
  for wanted_class in safe guided admin; do
    while IFS=$'\t' read -r class id _; do
      [ "$class" = "$wanted_class" ] || continue
      idx=$((idx+1))
      case " $raw " in *" $idx "*) wanted="${wanted}${wanted:+,}${id}";; esac
    done < "$DOCTOR_PLAN_FILE"
  done
  [ -n "$wanted" ] || return 1
  printf '%s\n' "$wanted"
}

doctor_plan_selection_hint(){
  local selectable repair manual
  selectable="$(doctor_plan_selectable_numbers)"
  repair="$(doctor_plan_action_ids repair)"
  manual="$(doctor_plan_action_ids manual)"
  [ -n "$selectable" ] && printf 'INFO Selectable repairs: %s\n' "$selectable"
  [ -n "$repair" ] && printf 'INFO Installer-only item(s) are not selectable here; run ~/install.sh --repair.\n'
  [ -n "$manual" ] && printf 'INFO Manual-review item(s) are not selectable; resolve the listed storage/system issue first.\n'
}

doctor_plan_post_repair_summary(){
  local class id title any=0
  [ -f "${DOCTOR_PLAN_FILE:-}" ] || return 0
  while IFS=$'\t' read -r class id title _; do
    case "$class" in
      repair)
        [ "$any" = 1 ] || { printf '\nINFO Post-repair status: additional action is still required.\n'; any=1; }
        printf 'INFO Remaining installer repair: %s — run ~/install.sh --repair\n' "$title"
        ;;
      manual)
        [ "$any" = 1 ] || { printf '\nINFO Post-repair status: additional action is still required.\n'; any=1; }
        printf 'INFO Remaining manual review: %s — Doctor will not write through a potentially unsafe condition.\n' "$title"
        ;;
    esac
  done < "$DOCTOR_PLAN_FILE"
  [ "$any" = 1 ] && return 1
  return 0
}

doctor_plan_prompt(){
  local choice selected
  if ! doctor_plan_has; then return 0; fi
  printf '\nChoose: [a] Apply safe repairs  [s] Select numbered repairs  [d] Details  [N] Cancel\n> '
  read -r choice || return 0
  case "$choice" in
    a|A|all|ALL)
      selected="$(doctor_plan_action_ids safe)"
      [ -n "$selected" ] || { printf 'INFO No automatic safe repairs are available.\n'; return 0; }
      exec "$0" --full --fix --only "$selected" --no-prompt
      ;;
    s|S|select|SELECT)
      doctor_plan_selection_hint
      printf 'Enter numbered safe/guided/admin repairs, separated by spaces or commas: '
      read -r choice || return 0
      selected="$(doctor_plan_ids_from_numbers "$choice" 2>/dev/null || true)"
      [ -n "$selected" ] || { warn "No selectable repair numbers were provided; no changes were made. Installer/manual plan items are marked [info]."; doctor_plan_selection_hint; return 0; }
      exec "$0" --full --fix --only "$selected" --no-prompt
      ;;
    d|D|details|DETAILS)
      doctor_plan_render 1
      doctor_plan_prompt
      ;;
    *) printf 'INFO No changes were made.\n' ;;
  esac
}

doctor_plan_interactive(){ doctor_plan_render; doctor_plan_prompt; }
