package discovery

import (
	"context"
	"time"

	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/go-utils/contextutils"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
)

type syncer struct {
	eds         *EndpointDiscovery
	refreshRate time.Duration
	discOpts    Opts
}

func NewEdsSyncer(disc *EndpointDiscovery, discOpts Opts, refreshRate time.Duration) v1.EdsSyncer {
	s := &syncer{
		eds:         disc,
		refreshRate: refreshRate,
		discOpts:    discOpts,
	}
	return s
}

func (s *syncer) Sync(ctx context.Context, snap *v1.EdsSnapshot) error {
	ctx = contextutils.WithLogger(ctx, "syncer")
	logger := contextutils.LoggerFrom(ctx)
	logger.Infof("begin sync %v (%v upstreams)", snap.Hash(), len(snap.Upstreams))
	defer logger.Infof("end sync %v", snap.Hash())

	logger.Debugf("%v", snap)

	opts := clients.WatchOpts{
		Ctx:         ctx,
		RefreshRate: s.refreshRate,
	}

	udsErrs, err := s.eds.StartEds(snap.Upstreams, opts)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case err := <-udsErrs:
				contextutils.LoggerFrom(ctx).Errorf("error in EDS: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
