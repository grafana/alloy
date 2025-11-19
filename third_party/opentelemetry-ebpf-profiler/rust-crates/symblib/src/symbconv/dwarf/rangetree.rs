// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use crate::VirtAddr;
use std::collections::VecDeque;
use std::{iter, ops};

/// References a sub-range of a tree node.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RangeTreeRef<'tree, T> {
    /// Sub-range being referenced. Not empty.
    pub range: ops::Range<VirtAddr>,

    /// Depth of the referenced node within the tree.
    pub depth: usize,

    /// Reference to the value of the node.
    pub value: &'tree T,
}

/// Specialized range tree structure.
///
/// All children in each node must:
/// - not overlap with any other child
/// - not be empty
/// - be fully covered by the parent range
/// - be sorted ascending by range (start, end)
#[derive(Debug, Clone)]
pub struct RangeTree<T> {
    /// Range covered by this node. Must not be empty.
    pub range: ops::Range<VirtAddr>,

    /// Value associated with this node.
    pub value: T,

    /// List of child nodes.
    ///
    /// All children must be sub-ranges of [`Self::range`] and cannot
    /// overlap each other. The list must be sorted ascending by range
    /// start. Ranges that start at the same offset must be sorted by
    /// ascending range end.
    pub children: Vec<RangeTree<T>>,
}

impl<T> RangeTree<T> {
    /// Recursively sort children to arrive at the required ordering guarantees.
    pub fn sort(&mut self) {
        self.children
            .sort_unstable_by_key(|x| (x.range.start, x.range.end));

        for child in &mut self.children {
            child.sort();
        }
    }

    /// Collects a flat list of the most specific nodes covering each range.
    ///
    /// The most specific node is the deepest child node covering the range. If
    /// you imagine the tree to be represented as a flame graph projected into
    /// 3D space, this would essentially represent the view from the top.
    ///
    /// For example, the tree
    ///
    /// ```text
    /// Depth
    /// 2 ┃    [ malloc ][ strcpy ]
    /// 1 ┃ [ strdup                  ]      [ puts ]
    /// 0 ┃ [ main                                       ]
    /// ━━╋━━━━━━━━━━━━━━━━━━━━━┷━━━━━━━━━━━━━━━━━━━━┷━━━━━ VA
    /// 0x000                 0x100                0x200
    /// ```
    ///
    /// would result in this result to be returned:
    ///
    /// ```text
    ///    strdup                 strdup              main
    ///      ↓                      ↓                  ↓
    ///     [ ][ malloc ][ strcpy ][  ][main][ puts ][   ]
    /// ━━╋━━━━━━━━━━━━━━━━━━━━━┷━━━━━━━━━━━━━━━━━━━━┷━━━━━ VA
    /// 0x000                 0x100                0x200
    /// ```
    pub fn collect_top_level_ranges(&self) -> Vec<RangeTreeRef<'_, T>> {
        let mut out = Vec::with_capacity(128);
        self.collect_top_level_ranges_rec(&mut out, 0);
        out
    }

    fn collect_top_level_ranges_rec<'tree>(
        &'tree self,
        out: &mut Vec<RangeTreeRef<'tree, T>>,
        depth: usize,
    ) {
        out.push(RangeTreeRef {
            range: self.range.clone(),
            depth,
            value: &self.value,
        });

        for child in &self.children {
            let prev = out.last_mut().unwrap();
            debug_assert!(prev.range.end >= prev.range.start);
            debug_assert!(prev.range.end >= child.range.end);
            debug_assert!(child.range.start >= prev.range.start);

            // Truncate previous node: the child takes precedence.
            prev.range.end = child.range.start;

            // If the truncation caused the previous node to be empty, get rid of it.
            if prev.range.is_empty() {
                out.pop();
            }

            // Recurse into children.
            child.collect_top_level_ranges_rec(out, depth + 1);

            // If the child doesn't fully cover our range to end, insert ourself again.
            let prev = out.last().unwrap();
            if self.range.end > prev.range.end {
                out.push(RangeTreeRef {
                    range: prev.range.end..self.range.end,
                    depth,
                    value: &self.value,
                });
            } else {
                debug_assert_eq!(self.range.end, prev.range.end);
            }
        }
    }

    /// Finds the matching tree node at the given depth.
    pub fn find_match_at_depth_mut(
        &mut self,
        at_depth: u64,
        rng: ops::Range<VirtAddr>,
    ) -> Option<&mut RangeTree<T>> {
        fn is_sub_range(outer: &ops::Range<VirtAddr>, sub: &ops::Range<VirtAddr>) -> bool {
            sub.start >= outer.start && sub.end <= outer.end
        }

        if at_depth == 0 {
            return if is_sub_range(&self.range, &rng) {
                Some(self)
            } else {
                None
            };
        }

        let mut node = self;
        'outer: for depth in 1..=at_depth {
            for child in &mut node.children {
                if is_sub_range(&child.range, &rng) {
                    node = child;

                    if depth == at_depth {
                        return Some(node);
                    }

                    continue 'outer;
                }
            }

            break;
        }

        None
    }

    /// Iterate over the tree's items, in depth-first order.
    pub fn iter_dfs(&self) -> impl Iterator<Item = &Self> {
        let mut queue = VecDeque::from([self]);

        iter::from_fn(move || {
            let node = queue.pop_back()?;
            queue.extend(node.children.iter().rev());
            Some(node)
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_test_tree() -> RangeTree<i32> {
        RangeTree {
            range: 0x100..0x1000,
            value: 1,
            children: vec![
                RangeTree {
                    range: 0x200..0x300,
                    value: 2,
                    children: vec![],
                },
                RangeTree {
                    range: 0x400..0x700,
                    value: 3,
                    children: vec![],
                },
            ],
        }
    }

    #[test]
    fn fn_tree_node() {
        let tree = make_test_tree();

        let expected = [
            RangeTreeRef {
                range: 0x100..0x200,
                depth: 0,
                value: &1,
            },
            RangeTreeRef {
                range: 0x200..0x300,
                depth: 1,
                value: &2,
            },
            RangeTreeRef {
                range: 0x300..0x400,
                depth: 0,
                value: &1,
            },
            RangeTreeRef {
                range: 0x400..0x700,
                depth: 1,
                value: &3,
            },
            RangeTreeRef {
                range: 0x700..0x1000,
                depth: 0,
                value: &1,
            },
        ];

        let flat = tree.collect_top_level_ranges();
        assert_eq!(flat.len(), expected.len());
        for (actual, expected) in iter::zip(flat, expected) {
            assert_eq!(actual, expected);
        }
    }

    #[test]
    fn find_match_at_depth() {
        let mut tree = make_test_tree();

        let node = tree.find_match_at_depth_mut(0, 0x30..0x50);
        assert!(node.is_none());

        let node = tree.find_match_at_depth_mut(0, 0x110..0x150).unwrap();
        assert_eq!(node.value, 1);

        let node = tree.find_match_at_depth_mut(1, 0x210..0x250).unwrap();
        assert_eq!(node.value, 2);

        let node = tree.find_match_at_depth_mut(0, 0x200..0x300).unwrap();
        assert_eq!(node.value, 1);

        let node = tree.find_match_at_depth_mut(1, 0x200..0x300).unwrap();
        assert_eq!(node.value, 2);

        let node = tree.find_match_at_depth_mut(3, 0x200..0x300);
        assert!(node.is_none());

        let node = tree.find_match_at_depth_mut(0, 0x310..0x330).unwrap();
        assert_eq!(node.value, 1);
    }
}
