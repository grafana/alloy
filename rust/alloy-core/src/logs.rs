//! Log batches passed between Go and Rust, in both directions.
//!
//! Wire format, all integers little endian:
//!
//! ```text
//! batch  = u32 stream count, streams
//! stream = labels, u32 entry count, entries
//! entry  = i64 unix nanos, bytes line, labels structured metadata
//! labels = u32 count, (bytes name, bytes value) per label
//! bytes  = u32 length, data
//! ```

use std::borrow::Cow;

use crate::Error;

/// A batch of log streams. Batches read from Go borrow from the buffer they
/// were read from; batches built in Rust are usually `LogBatch<'static>`.
#[derive(Debug, Default)]
pub struct LogBatch<'a> {
    pub streams: Vec<LogStream<'a>>,
}

#[derive(Debug, Default)]
pub struct LogStream<'a> {
    pub labels: Vec<Label<'a>>,
    pub entries: Vec<LogEntry<'a>>,
}

#[derive(Debug)]
pub struct LogEntry<'a> {
    pub ts_nanos: i64,
    pub line: Cow<'a, str>,
    pub structured_metadata: Vec<Label<'a>>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Label<'a> {
    pub name: Cow<'a, str>,
    pub value: Cow<'a, str>,
}

impl<'a> LogEntry<'a> {
    pub fn new(ts_nanos: i64, line: impl Into<Cow<'a, str>>) -> Self {
        Self {
            ts_nanos,
            line: line.into(),
            structured_metadata: Vec::new(),
        }
    }
}

impl<'a> Label<'a> {
    pub fn new(name: impl Into<Cow<'a, str>>, value: impl Into<Cow<'a, str>>) -> Self {
        Self {
            name: name.into(),
            value: value.into(),
        }
    }
}

impl<'a> LogBatch<'a> {
    pub fn new() -> Self {
        Self::default()
    }

    /// Adds entry to the stream with exactly these labels, creating it if
    /// needed.
    pub fn push(&mut self, labels: &[Label<'a>], entry: LogEntry<'a>) {
        match self.streams.iter_mut().find(|s| s.labels == labels) {
            Some(stream) => stream.entries.push(entry),
            None => self.streams.push(LogStream {
                labels: labels.to_vec(),
                entries: vec![entry],
            }),
        }
    }

    pub fn encode(&self) -> Vec<u8> {
        let mut w = Writer { buf: Vec::new() };
        w.u32(self.streams.len());
        for stream in &self.streams {
            w.labels(&stream.labels);
            w.u32(stream.entries.len());
            for entry in &stream.entries {
                w.buf.extend_from_slice(&entry.ts_nanos.to_le_bytes());
                w.str(&entry.line);
                w.labels(&entry.structured_metadata);
            }
        }
        w.buf
    }

    /// Reads a batch. Invalid UTF-8 is replaced, as Go strings don't have to be
    /// valid UTF-8.
    pub fn read(buf: &'a [u8]) -> Result<Self, Error> {
        let mut r = Reader { buf };
        let streams = (0..r.u32()?)
            .map(|_| {
                let labels = r.labels()?;
                let entries = (0..r.u32()?)
                    .map(|_| {
                        Ok(LogEntry {
                            ts_nanos: r.i64()?,
                            line: r.str()?,
                            structured_metadata: r.labels()?,
                        })
                    })
                    .collect::<Result<_, Error>>()?;
                Ok(LogStream { labels, entries })
            })
            .collect::<Result<_, Error>>()?;

        if !r.buf.is_empty() {
            return Err(format!("{} trailing bytes after batch", r.buf.len()).into());
        }
        Ok(LogBatch { streams })
    }
}

struct Reader<'a> {
    buf: &'a [u8],
}

impl<'a> Reader<'a> {
    fn take(&mut self, n: usize) -> Result<&'a [u8], Error> {
        if self.buf.len() < n {
            return Err("batch is truncated".into());
        }
        let (head, rest) = self.buf.split_at(n);
        self.buf = rest;
        Ok(head)
    }

    fn u32(&mut self) -> Result<u32, Error> {
        Ok(u32::from_le_bytes(self.take(4)?.try_into().unwrap()))
    }

    fn i64(&mut self) -> Result<i64, Error> {
        Ok(i64::from_le_bytes(self.take(8)?.try_into().unwrap()))
    }

    fn str(&mut self) -> Result<Cow<'a, str>, Error> {
        let n = self.u32()? as usize;
        Ok(String::from_utf8_lossy(self.take(n)?))
    }

    fn labels(&mut self) -> Result<Vec<Label<'a>>, Error> {
        (0..self.u32()?)
            .map(|_| {
                Ok(Label {
                    name: self.str()?,
                    value: self.str()?,
                })
            })
            .collect()
    }
}

struct Writer {
    buf: Vec<u8>,
}

impl Writer {
    fn u32(&mut self, n: usize) {
        self.buf.extend_from_slice(&(n as u32).to_le_bytes());
    }

    fn str(&mut self, s: &str) {
        self.u32(s.len());
        self.buf.extend_from_slice(s.as_bytes());
    }

    fn labels(&mut self, labels: &[Label<'_>]) {
        self.u32(labels.len());
        for l in labels {
            self.str(&l.name);
            self.str(&l.value);
        }
    }
}
