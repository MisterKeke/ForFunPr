package service

import "github.com/wailsapp/wails/v2/pkg/runtime"

const (
	desktopAppsChangedEvent           = "desktop-apps:changed"
	setupsChangedEvent                = "setups:changed"
	steamGamesChangedEvent            = "steam-games:changed"
	wallpapersChangedEvent            = "wallpapers:changed"
	channelPostsChangedEvent          = "channel-posts:changed"
	gitWorkspaceInventoryChangedEvent = "git-workspaces:inventory-changed"
	gitWorkspaceStatusUpdatedEvent    = "git-workspaces:status-updated"
	gitWorkspaceJobProgressEvent      = "git-workspaces:job-progress"
	gitWorkspaceJobCompleteEvent      = "git-workspaces:job-complete"
)

// These emitters are called by the REST layer after mutations originating
// outside the Wails UI. Direct Wails calls already consume their canonical
// return values, so they intentionally do not emit duplicate events.
func (a *Service) EmitDesktopAppsChanged() {
	a.emitChangeEvent(desktopAppsChangedEvent)
}

func (a *Service) EmitSetupsChanged() {
	a.emitChangeEvent(setupsChangedEvent)
}

func (a *Service) EmitSteamGamesChanged() {
	a.emitChangeEvent(steamGamesChangedEvent)
}

func (a *Service) EmitWallpapersChanged() {
	a.emitChangeEvent(wallpapersChangedEvent)
}

func (a *Service) EmitChannelPostsChanged(source string, channel string) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, channelPostsChangedEvent, map[string]string{
			"source":  source,
			"channel": channel,
		})
	}
}

func (a *Service) EmitGitWorkspaceInventoryChanged() {
	a.emitGitWorkspaceInventoryChanged()
}

func (a *Service) EmitGitWorkspaceStatusUpdated(repositoryID int) {
	a.emitGitWorkspaceStatusUpdated(repositoryID)
}

func (a *Service) EmitGitWorkspaceJobProgress(job GitWorkspaceJob) {
	a.emitGitWorkspaceJobProgress(job)
}

func (a *Service) EmitGitWorkspaceJobComplete(job GitWorkspaceJob) {
	a.emitGitWorkspaceJobComplete(job)
}

func (a *Service) emitGitWorkspaceInventoryChanged() {
	a.emitChangeEvent(gitWorkspaceInventoryChangedEvent)
}

func (a *Service) emitGitWorkspaceStatusUpdated(repositoryID int) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, gitWorkspaceStatusUpdatedEvent, map[string]int{"repository_id": repositoryID})
	}
}

func (a *Service) emitGitWorkspaceJobProgress(job GitWorkspaceJob) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, gitWorkspaceJobProgressEvent, job)
	}
}

func (a *Service) emitGitWorkspaceJobComplete(job GitWorkspaceJob) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, gitWorkspaceJobCompleteEvent, job)
	}
}

func (a *Service) emitChangeEvent(name string) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, name)
	}
}
