package workers

// Reaper is superseded by Claimer (XAUTOCLAIM + stuck-running safety net).
// Kept as a type alias so older call sites compile during the switch.
type Reaper = Claimer
