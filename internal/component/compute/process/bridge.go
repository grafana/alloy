package process

// bridge handles the passthrough
type bridge struct {
	lr   *lokiReceiver
	prom *bulkAppendable
}

func (b *bridge) sendPassthrough(pt *Passthrough) {
	if len(pt.Lokilogs) > 0 {
		b.lr.send(pt.Lokilogs)
	}
	if len(pt.Prommetrics) > 0 {
		b.prom.send(pt.Prommetrics)
	}
}
