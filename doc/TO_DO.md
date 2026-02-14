# Space Optimization

### Plan ID

The number of feature-set runs and coupled plan IDs for an active buffer will usually be very low.
The current implementation holds a `uint16` for every glyph position, which is wasteful.
Idea: use a run-length encoding or array of “pointer” indices.
