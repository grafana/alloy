use std::time::{Duration, SystemTime, UNIX_EPOCH};

use alloy_core::schema::{Field, FieldType, Value};
use alloy_core::{Arguments, Batch, Component, Context, Error, Output, Source};

pub struct Args {
    pub lines_per_batch: u32,
    pub interval: Duration,
}

#[repr(C)]
#[derive(Clone, Copy)]
pub struct ArgsRaw {
    pub lines_per_batch: u32,
    pub interval_ms: u64,
}

impl Arguments for Args {
    type Raw = ArgsRaw;

    fn from_raw(raw: &ArgsRaw) -> Result<Self, Error> {
        Ok(Self {
            lines_per_batch: raw.lines_per_batch,
            interval: Duration::from_millis(raw.interval_ms),
        })
    }

    fn schema() -> &'static [Field] {
        const SCHEMA: &[Field] = &[
            Field {
                name: "lines_per_batch",
                ty: FieldType::U32,
                default: Some(Value::Uint(100)),
                min: Some(Value::Uint(1)),
                max: None,
                doc: "Number of lines in each batch.",
            },
            Field {
                name: "interval",
                ty: FieldType::DurationMs,
                default: Some(Value::Duration(Duration::from_secs(1))),
                min: Some(Value::Duration(Duration::from_millis(1))),
                max: None,
                doc: "How often to generate a batch.",
            },
        ];
        SCHEMA
    }
}

pub struct Rustgen {
    args: Args,
    counter: u64,
}

impl Component for Rustgen {
    type Args = Args;

    fn new(args: Args) -> Result<Self, Error> {
        Ok(Self { args, counter: 0 })
    }
}

impl Source for Rustgen {
    fn run(&mut self, ctx: &Context, out: &Output) {
        while ctx.sleep(self.args.interval) {
            let ts = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_nanos() as i64;

            let mut batch = Batch::new();
            for _ in 0..self.args.lines_per_batch {
                batch.push(ts, &format!("rustgen line {}", self.counter));
                self.counter += 1;
            }

            if out.send(batch).is_err() {
                return;
            }
        }
    }
}
