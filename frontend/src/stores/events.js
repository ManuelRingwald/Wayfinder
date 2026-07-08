import { defineStore } from 'pinia'
import { ref } from 'vue'

// Bounded ring buffer for the Alarm-/Ereignis-Panel (ASD-013). Holds the most
// recent operator-facing events (newest first). The cap bounds memory on a
// long-running scope with a busy feed; older events roll off the end.
export const MAX_EVENTS = 200

// useEventsStore keeps the event log and an unseen counter (for the bell badge).
// The pure derivation lives in map/events.js; this store only stamps identity +
// timestamp and enforces the buffer bound. Every WS (re)connect re-scopes the
// stream, so the log is implicitly tenant-scoped (WF2-21).
export const useEventsStore = defineStore('events', () => {
  const events = ref([]) // newest first: [{ id, ts, type, severity, message, trackNum? }, …]
  const unseenCount = ref(0)
  let nextId = 1

  // add records one derived event ({ type, severity, message, trackNum? }),
  // stamping a monotonic id and a wall-clock timestamp, prepending it and
  // trimming to MAX_EVENTS. Returns the stored record.
  function add(evt) {
    const record = { id: nextId++, ts: Date.now(), ...evt }
    const next = [record, ...events.value]
    if (next.length > MAX_EVENTS) next.length = MAX_EVENTS
    events.value = next
    unseenCount.value += 1
    return record
  }

  // addMany records a batch in order (e.g. all lifecycle events of one WS frame).
  function addMany(evts) {
    for (const e of evts || []) add(e)
  }

  // clear empties the log and resets the unseen counter (operator "Leeren").
  function clear() {
    events.value = []
    unseenCount.value = 0
  }

  // markSeen resets the unseen counter without dropping history — called when the
  // operator opens the panel.
  function markSeen() {
    unseenCount.value = 0
  }

  return { events, unseenCount, add, addMany, clear, markSeen }
})
