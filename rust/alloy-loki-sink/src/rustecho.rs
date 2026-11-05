use alloy_core::logs::{Label, LogBatch};
use alloy_core::{Component, Error, Sink};

#[alloy_core::arguments]
pub struct Args {}

/// Prints every entry it receives to stderr.
#[alloy_core::component(name = "loki.rustecho", stability = Experimental)]
pub struct Rustecho;

impl Component for Rustecho {
    type Args = Args;

    fn new(_args: Args) -> Result<Self, Error> {
        Ok(Self)
    }
}

impl Sink for Rustecho {
    fn consume(&self, batch: &LogBatch<'_>) -> Result<(), Error> {
        for stream in &batch.streams {
            let labels = format_labels(&stream.labels);
            for entry in &stream.entries {
                eprintln!(
                    "rustecho {labels} {} {} {}",
                    entry.ts_nanos,
                    entry.line,
                    format_labels(&entry.structured_metadata)
                );
            }
        }
        Ok(())
    }
}

fn format_labels(labels: &[Label<'_>]) -> String {
    let labels: Vec<_> = labels
        .iter()
        .map(|l| format!("{}={:?}", l.name, l.value))
        .collect();
    format!("{{{}}}", labels.join(", "))
}
