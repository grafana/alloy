// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Internal helper types for string table construction during writing.

use std::collections::HashMap;

/// Temporary index handed out while table is being built.
///
/// Values should be considered opaque / an implementation details.
pub type TempIdx = u32;

/// Index in the final table.
///
/// Values are indices into the final `Vec<String>` table.
pub type Idx = u32;

/// Translates temporary indices to the final ones.
#[derive(Debug)]
pub struct Mapper {
    /// Map of unique strings that should be inlined as strings.
    unique: HashMap<TempIdx, String>,

    /// Mapping from temporary to final index after removing unique strings.
    translation: HashMap<TempIdx, Idx>,

    /// Actual string table.
    table: Vec<String>,
}

impl Mapper {
    /// Translate a given temporary index to the final array position or return
    /// the string if it wasn't actually duplicated and should be inlined.
    ///
    /// This is a destructive action: unique strings are taken out of the table
    /// on the first call for their index.
    pub fn translate(&mut self, old_idx: TempIdx) -> Mapping {
        if let Some(unique) = self.unique.remove(&old_idx) {
            return Mapping::Unique(unique);
        }

        if let Some(new_idx) = self.translation.get(&old_idx) {
            return Mapping::Translate(*new_idx);
        }

        unreachable!("bug: invalid index passed to internal `translate` function")
    }

    /// Forces an entry into the string table even if it is unique.
    pub fn force_entry(&mut self, old_idx: TempIdx) -> Idx {
        match self.translate(old_idx) {
            // String was previously not in the table due to being unique: move.
            Mapping::Unique(s) => {
                let new_idx = self.table.len() as Idx;
                self.table.push(s);
                self.translation.insert(old_idx, new_idx);
                new_idx
            }

            // Entry exists already: just return new index.
            Mapping::Translate(idx) => idx,
        }
    }

    /// Consume the translator, returning the final table.
    pub fn into_table(self) -> Vec<String> {
        self.table
    }
}

/// Mapping type
#[derive(Debug, PartialEq, Eq)]
pub enum Mapping {
    /// String is unique within current sequence: inline it.
    Unique(String),

    /// Replace old index with the given new index.
    Translate(Idx),
}

/// Incrementally de-duplicates strings.
#[derive(Debug, Default)]
pub struct Builder {
    entries: HashMap<String, BuilderEntry>,
    size_estimate: usize,
    next_id: TempIdx,
}

impl Builder {
    /// Look up or construct the temporary ID for the given string.
    pub fn index_for_str(&mut self, s: String) -> TempIdx {
        match self.entries.get_mut(&s) {
            Some(entry) => {
                entry.count += 1;
                entry.id
            }
            None => {
                // 5 = maximum length of var-int u32
                self.size_estimate += 5 + s.len();
                let id = self.next_id;
                let entry = BuilderEntry { id, count: 1 };
                self.entries.insert(s, entry);
                self.next_id += 1;
                id
            }
        }
    }

    /// Estimated serialized size of the string table, in bytes.
    pub fn size_estimate(&self) -> usize {
        self.size_estimate
    }

    /// Consume the builder, constructing the final string table.
    pub fn build(self) -> Mapper {
        let (duped, unique): (Vec<_>, Vec<_>) =
            self.entries.into_iter().partition(|x| x.1.count > 1);

        let unique: HashMap<_, _> = unique
            .into_iter()
            .map(|entry| (entry.1.id, entry.0))
            .collect();

        let translation: HashMap<_, _> = duped
            .iter()
            .enumerate()
            .map(|(new_idx, entry)| (entry.1.id, new_idx as Idx))
            .collect();

        let table = duped.into_iter().map(|x| x.0).collect();

        Mapper {
            unique,
            translation,
            table,
        }
    }
}

#[derive(Debug)]
struct BuilderEntry {
    id: TempIdx,
    count: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dedup() {
        let mut builder = Builder::default();

        let id_abc = builder.index_for_str("abc".into());
        let id_bcd = builder.index_for_str("bcd".into());
        assert_eq!(id_abc, builder.index_for_str("abc".into()));
        assert_ne!(id_abc, id_bcd);
        let id_xyz = builder.index_for_str("xyz".into());

        let mut mapper = builder.build();
        assert_eq!(mapper.translate(id_abc), Mapping::Translate(0));
        assert_eq!(mapper.force_entry(id_xyz), 1);
        assert_eq!(mapper.translate(id_bcd), Mapping::Unique("bcd".to_owned()));

        let table = mapper.into_table();
        assert_eq!(table, &["abc", "xyz"]);
    }
}
