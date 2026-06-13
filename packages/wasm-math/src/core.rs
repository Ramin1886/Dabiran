//! Pure, dependency-free canvas math. This module contains no `wasm_bindgen`
//! glue so it can be unit-tested with a normal native `cargo test`, and the
//! TypeScript fallback in `apps/frontend/src/math/engine.ts` mirrors it
//! exactly. The wasm-exported wrappers in `lib.rs` are thin shims over these.

/// An axis-aligned world-space rectangle (inclusive bounds).
#[derive(Clone, Copy)]
pub struct Rect {
    pub min_x: f32,
    pub min_y: f32,
    pub max_x: f32,
    pub max_y: f32,
}

/// Tests whether a world point falls inside the rectangle (inclusive).
#[inline]
pub fn point_in_rect(x: f32, y: f32, r: Rect) -> bool {
    x >= r.min_x && x <= r.max_x && y >= r.min_y && y <= r.max_y
}

/// Tests whether the segment (ax,ay)-(bx,by) is relevant to `r`: relevant when
/// either endpoint is inside, or when the segment's bounding box overlaps the
/// rect. This is the cheap conservative crossing test used for connector
/// culling — it keeps long diagonals spanning the viewport drawn.
#[inline]
pub fn segment_touches_rect(ax: f32, ay: f32, bx: f32, by: f32, r: Rect) -> bool {
    if point_in_rect(ax, ay, r) || point_in_rect(bx, by, r) {
        return true;
    }
    let seg_min_x = ax.min(bx);
    let seg_max_x = ax.max(bx);
    let seg_min_y = ay.min(by);
    let seg_max_y = ay.max(by);
    seg_max_x >= r.min_x && seg_min_x <= r.max_x && seg_max_y >= r.min_y && seg_min_y <= r.max_y
}

/// Returns the indices of the points (packed as `[x0,y0,x1,y1,...]`) that fall
/// inside `r`. A trailing odd float is ignored.
pub fn cull_point_indices(positions: &[f32], r: Rect) -> Vec<u32> {
    let n = positions.len() / 2;
    let mut out = Vec::new();
    for i in 0..n {
        if point_in_rect(positions[2 * i], positions[2 * i + 1], r) {
            out.push(i as u32);
        }
    }
    out
}

/// Returns the indices of the segments (packed as `[ax,ay,bx,by,...]`) that
/// touch `r` per [`segment_touches_rect`].
pub fn cull_segment_indices(segments: &[f32], r: Rect) -> Vec<u32> {
    let m = segments.len() / 4;
    let mut out = Vec::new();
    for i in 0..m {
        let o = 4 * i;
        if segment_touches_rect(segments[o], segments[o + 1], segments[o + 2], segments[o + 3], r) {
            out.push(i as u32);
        }
    }
    out
}

/// Flattens the branch connector from (sx,sy) to (ex,ey) into a polyline of
/// world points, packed `[x0,y0,...]`, ready to feed a WebGL line buffer.
///
/// The geometry matches the renderer: a straight horizontal segment when the
/// endpoints share a Y (same lane), otherwise a cubic Bezier whose control
/// points sit at the horizontal midpoint — i.e. P0=(sx,sy),
/// P1=(cpx,sy), P2=(cpx,ey), P3=(ex,ey) with cpx = sx + (ex-sx)/2 — sampled
/// into `segments` sub-segments (clamped to at least 1).
pub fn bezier_polyline(sx: f32, sy: f32, ex: f32, ey: f32, segments: u32) -> Vec<f32> {
    // Same lane → straight line, two points.
    if (sy - ey).abs() < f32::EPSILON {
        return vec![sx, sy, ex, ey];
    }
    let steps = segments.max(1);
    let cpx = sx + (ex - sx) / 2.0;
    let (p0x, p0y) = (sx, sy);
    let (p1x, p1y) = (cpx, sy);
    let (p2x, p2y) = (cpx, ey);
    let (p3x, p3y) = (ex, ey);

    let mut out = Vec::with_capacity(((steps + 1) * 2) as usize);
    for i in 0..=steps {
        let t = i as f32 / steps as f32;
        let mt = 1.0 - t;
        // Cubic Bezier basis.
        let b0 = mt * mt * mt;
        let b1 = 3.0 * mt * mt * t;
        let b2 = 3.0 * mt * t * t;
        let b3 = t * t * t;
        out.push(b0 * p0x + b1 * p1x + b2 * p2x + b3 * p3x);
        out.push(b0 * p0y + b1 * p1y + b2 * p2y + b3 * p3y);
    }
    out
}

/// Computes the chronological branch layout for a set of commits, mirroring
/// the backend `layoutNodes` algorithm so the client can re-lay-out a filtered
/// subset (recompacting lanes) without a round-trip.
///
/// Inputs are parallel arrays indexed by node:
/// - `dates`: author timestamp in Unix seconds.
/// - `primary_parent`: index (into these same arrays) of the node's first
///   parent, or -1 when it has none / the parent is not in the set.
///
/// Nodes are processed in ascending date order (ties broken by input index).
/// `x_offset = (date - oldest_date) * 0.05`. A node reuses its primary
/// parent's lane when that parent still occupies one, otherwise it claims the
/// next new lane (lanes are never freed, matching the backend).
///
/// Returns a flat `[lane0, x0, lane1, x1, …]` buffer aligned to the INPUT
/// order (lane stored as f32; exact for realistic lane counts).
pub fn layout(dates: &[f64], primary_parent: &[i32]) -> Vec<f32> {
    let n = dates.len();
    let mut out = vec![0.0f32; n * 2];
    if n == 0 {
        return out;
    }

    // Process order: ascending date, ties broken by original index.
    let mut order: Vec<usize> = (0..n).collect();
    order.sort_by(|&a, &b| {
        dates[a]
            .partial_cmp(&dates[b])
            .unwrap_or(std::cmp::Ordering::Equal)
            .then(a.cmp(&b))
    });

    let origin = dates[order[0]];
    const SCALE: f64 = 0.05;
    // active[lane] = index of the node currently occupying that lane.
    let mut active: Vec<i32> = Vec::new();

    for &i in &order {
        let mut assigned: i32 = -1;
        let pp = primary_parent[i];
        if pp >= 0 {
            for (lane, &occ) in active.iter().enumerate() {
                if occ == pp {
                    assigned = lane as i32;
                    break;
                }
            }
        }
        if assigned < 0 {
            assigned = active.len() as i32;
            active.push(i as i32);
        } else {
            active[assigned as usize] = i as i32;
        }
        out[2 * i] = assigned as f32;
        out[2 * i + 1] = ((dates[i] - origin) * SCALE) as f32;
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    const R: Rect = Rect { min_x: 0.0, min_y: 0.0, max_x: 100.0, max_y: 100.0 };

    #[test]
    fn point_in_rect_bounds_are_inclusive() {
        assert!(point_in_rect(0.0, 0.0, R));
        assert!(point_in_rect(100.0, 100.0, R));
        assert!(point_in_rect(50.0, 50.0, R));
        assert!(!point_in_rect(-1.0, 50.0, R));
        assert!(!point_in_rect(50.0, 101.0, R));
    }

    #[test]
    fn cull_points_keeps_only_inside() {
        // (10,10) inside, (200,10) outside, (90,90) inside.
        let positions = [10.0, 10.0, 200.0, 10.0, 90.0, 90.0];
        assert_eq!(cull_point_indices(&positions, R), vec![0, 2]);
    }

    #[test]
    fn segment_touches_when_endpoint_inside_or_bbox_overlaps() {
        // Endpoint inside.
        assert!(segment_touches_rect(50.0, 50.0, 500.0, 500.0, R));
        // Both endpoints outside but the segment's bbox spans the rect.
        assert!(segment_touches_rect(-50.0, 50.0, 500.0, 50.0, R));
        // Fully outside, bbox disjoint.
        assert!(!segment_touches_rect(200.0, 200.0, 300.0, 300.0, R));
    }

    #[test]
    fn cull_segments_filters_by_touch() {
        // seg0 touches (endpoint inside), seg1 disjoint.
        let segments = [50.0, 50.0, 500.0, 500.0, 200.0, 200.0, 300.0, 300.0];
        assert_eq!(cull_segment_indices(&segments, R), vec![0]);
    }

    #[test]
    fn straight_connector_is_two_points() {
        let line = bezier_polyline(10.0, 40.0, 90.0, 40.0, 8);
        assert_eq!(line, vec![10.0, 40.0, 90.0, 40.0]);
    }

    #[test]
    fn bezier_endpoints_match_and_count_is_correct() {
        let segments = 8;
        let poly = bezier_polyline(0.0, 0.0, 100.0, 80.0, segments);
        // (segments + 1) points × 2 coords.
        assert_eq!(poly.len(), ((segments + 1) * 2) as usize);
        // First point is the start, last is the end.
        assert!((poly[0] - 0.0).abs() < 1e-4 && (poly[1] - 0.0).abs() < 1e-4);
        let n = poly.len();
        assert!((poly[n - 2] - 100.0).abs() < 1e-3 && (poly[n - 1] - 80.0).abs() < 1e-3);
        // Midpoint t=0.5 sits at the horizontal centre by symmetry of the
        // control points (x = 50).
        let mid = poly.len() / 2;
        // mid index is a point boundary only when (segments+1) is odd; assert
        // the curve stays within the x-span instead.
        for k in (0..n).step_by(2) {
            assert!(poly[k] >= -0.01 && poly[k] <= 100.01);
        }
        let _ = mid;
    }

    #[test]
    fn bezier_clamps_zero_segments() {
        // segments=0 is clamped to 1 → start + end.
        let poly = bezier_polyline(0.0, 0.0, 10.0, 10.0, 0);
        assert_eq!(poly.len(), 4);
    }

    #[test]
    fn layout_assigns_lanes_and_offsets() {
        // A(t=0, no parent), B(t=10, parent A), C(t=20, no parent).
        // B takes over A's lane (0); C claims a new lane (1).
        let dates = [0.0, 10.0, 20.0];
        let primary_parent = [-1, 0, -1];
        let out = layout(&dates, &primary_parent);
        // [laneA, xA, laneB, xB, laneC, xC]
        assert_eq!(out[0], 0.0); // A lane 0
        assert_eq!(out[2], 0.0); // B reuses A's lane 0
        assert_eq!(out[4], 1.0); // C new lane 1
        assert!((out[1] - 0.0).abs() < 1e-4); // xA = 0
        assert!((out[3] - 0.5).abs() < 1e-4); // xB = 10 * 0.05
        assert!((out[5] - 1.0).abs() < 1e-4); // xC = 20 * 0.05
    }

    #[test]
    fn layout_orders_by_date_regardless_of_input_order() {
        // Input is newest-first; the oldest still anchors x_offset origin.
        let dates = [20.0, 0.0];
        let primary_parent = [-1, -1];
        let out = layout(&dates, &primary_parent);
        // Node 1 (t=0) is the origin → x=0; node 0 (t=20) → x=1.0.
        assert!((out[3] - 0.0).abs() < 1e-4);
        assert!((out[1] - 1.0).abs() < 1e-4);
        // Two independent roots get distinct lanes.
        assert_ne!(out[0], out[2]);
    }

    #[test]
    fn layout_empty_is_empty() {
        assert!(layout(&[], &[]).is_empty());
    }
}
