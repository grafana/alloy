/// A batch of log entries for a single stream.
#[derive(Default)]
pub struct Batch {
    buf: Vec<u8>,
}

impl Batch {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn push(&mut self, ts_nanos: i64, line: &str) {
        // i64 LE unix nanos, u32 LE line length, line bytes.
        self.buf.extend_from_slice(&ts_nanos.to_le_bytes());
        self.buf.extend_from_slice(&(line.len() as u32).to_le_bytes());
        self.buf.extend_from_slice(line.as_bytes());
    }

    pub fn into_bytes(self) -> Vec<u8> {
        self.buf
    }
}
