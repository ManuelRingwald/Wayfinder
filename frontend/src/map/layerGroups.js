// Sidebar layer-group model (ASD-020, ADR 0031): the "Layer" section of the
// sidebar is organised into collapsible GROUPS, each with a tri-state master
// control. This module holds the schema-agnostic tri-state LOGIC so it is unit-
// testable without a Vuetify mount; the group MEMBERSHIP (which store keys sit
// in which group) lives in LayerFilterContent.vue, right next to the rows it
// renders, so there is one obvious place to add a new layer.

/**
 * masterState reduces a group's member visibilities to its master-control state.
 *
 * Only ENABLED members must be passed in: a disabled toggle (e.g. a weather
 * layer whose upstream source is not configured) is one the operator cannot
 * change, so counting it would pin the master to 'mixed' forever. The caller
 * filters those out before calling this.
 *
 * @param {boolean[]} values visibilities of the group's enabled members
 * @returns {'on'|'off'|'mixed'|'empty'} 'empty' when there is nothing to control
 */
export function masterState(values) {
  if (!values || values.length === 0) return 'empty'
  let on = 0
  for (const v of values) if (v) on++
  if (on === 0) return 'off'
  if (on === values.length) return 'on'
  return 'mixed'
}

/**
 * nextMaster returns the visibility to apply when the master is clicked. The
 * convention is FILL-then-clear: a group that is off OR only partially on
 * (mixed) turns fully ON — one click "selects the group" and completes it — and
 * only an already-fully-on group turns OFF. This matches the operator's mental
 * model that clicking a group header selects its layers (#315). The previous
 * rule (mixed → off) made a click on a partially-active group CLEAR it, which
 * read as "selecting Aeronautik deselects everything". 'empty' never reaches
 * here (the master is hidden when the group has nothing controllable).
 *
 * @param {'on'|'off'|'mixed'|'empty'} state current master state
 * @returns {boolean} the visibility to set on every enabled member
 */
export function nextMaster(state) {
  return state !== 'on'
}
