use std::time::{Duration, SystemTime, UNIX_EPOCH};

use alloy_core::{Batch, Component, Context, Error, Output, Source};

#[alloy_core::arguments]
pub struct Args {
    /// Number of lines in each batch.
    #[arg(default = 100, min = 1)]
    pub lines_per_batch: u32,

    /// How often to generate a batch.
    #[arg(default = "1s", min = "1ms")]
    pub interval: Duration,
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
