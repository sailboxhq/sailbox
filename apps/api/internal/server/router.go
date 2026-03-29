package server

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	v1 "github.com/sailboxhq/sailbox/apps/api/internal/api/v1"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/ws"
	"github.com/sailboxhq/sailbox/apps/api/internal/auth"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// RouterDeps holds dependencies required by the router.
type RouterDeps struct {
	Services    *service.Container
	JWTManager  *auth.JWTManager
	Orch        orchestrator.Orchestrator
	Store       store.Store
	SSEBroker   *ws.SSEBroker
	AppURL      string // Public URL of the Sailbox instance
	SetupSecret string // Secret for unauthenticated setup operations
	Logger      *slog.Logger
}

// NewRouter creates and configures the Gin engine with all routes.
func NewRouter(deps *RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Global middleware
	r.Use(
		middleware.Recovery(deps.Logger),
		middleware.Sentry(),
		middleware.Branding(),
		middleware.RequestID(),
		middleware.Logger(deps.Logger),
		middleware.CORS(),
	)

	// Health check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// WebSocket / SSE routes (auth via query param token)
	wsGroup := r.Group("/ws")
	wsGroup.Use(middleware.WSAuth(deps.JWTManager))
	{
		logsHandler := ws.NewLogsHandler(deps.Store, deps.Orch, deps.Logger)
		wsGroup.GET("/logs/:appId", logsHandler.Handle)

		wsGroup.GET("/events", deps.SSEBroker.ServeHTTP)

		nodeLogsHandler := ws.NewNodeLogsHandler(deps.Services.Node, deps.Logger)
		wsGroup.GET("/nodes/:id/logs", nodeLogsHandler.Handle)

		terminalHandler := ws.NewTerminalHandler(deps.Store, deps.Orch, deps.Logger)
		wsGroup.GET("/terminal/:appId", terminalHandler.Handle)
	}

	// API v1
	apiV1 := r.Group("/api/v1")
	{
		// Public routes
		authHandler := v1.NewAuthHandler(deps.Services.Auth)
		loginRL := middleware.RateLimit(10, 1*time.Minute)
		apiV1.GET("/auth/setup-status", authHandler.SetupStatus)
		apiV1.POST("/auth/register", loginRL, authHandler.Register)
		apiV1.POST("/auth/login", loginRL, authHandler.Login)
		apiV1.POST("/auth/refresh", authHandler.Refresh)

		// GitHub OAuth/Manifest (public - GitHub redirects here without JWT)
		githubOAuth := v1.NewGitHubOAuthHandler(deps.Store, deps.Services.Resource, deps.AppURL, deps.Logger)
		apiV1.GET("/auth/github/callback", githubOAuth.Callback)
		apiV1.GET("/auth/github/setup/callback", githubOAuth.SetupCallback)

		// Webhooks (public - called by GitHub/GitLab without JWT)
		webhookHandler := v1.NewWebhookHandler(deps.Store, deps.Services.Deploy, deps.Logger)
		apiV1.POST("/webhooks/github/:appId", webhookHandler.GitHub)
		apiV1.POST("/webhooks/gitlab/:appId", webhookHandler.GitLab)

		// Team invitations (public — invitee may not have an account yet)
		teamPublic := v1.NewTeamHandler(deps.Services.Team, deps.Services.Notification, deps.AppURL)
		apiV1.GET("/team/invitations/info", teamPublic.GetInvitationByToken)
		apiV1.POST("/team/invitations/accept-public", loginRL, teamPublic.AcceptInvitationPublic)

		// System restore (requires setup secret + only works on uninitialized system)
		restoreRL := middleware.RateLimit(5, 5*time.Minute)
		setupAuth := middleware.RequireSetupSecret(deps.SetupSecret)
		sysBackupPublic := v1.NewSystemBackupHandler(deps.Services.SystemBackup)
		apiV1.POST("/system/restore/scan", restoreRL, setupAuth, sysBackupPublic.ScanS3Backups)
		apiV1.POST("/system/restore/execute", restoreRL, setupAuth, sysBackupPublic.RestoreFromS3)

		// Protected routes
		protected := apiV1.Group("")
		protected.Use(middleware.Auth(deps.JWTManager))
		{
			// Auth
			protected.GET("/auth/me", authHandler.Me)
			protected.PATCH("/auth/profile", authHandler.UpdateProfile)
			protected.POST("/auth/change-password", authHandler.ChangePassword)
			protected.GET("/auth/avatars", authHandler.ListAvatars)
			protected.POST("/auth/2fa/setup", authHandler.Setup2FA)
			protected.POST("/auth/2fa/verify", authHandler.Verify2FA)
			protected.POST("/auth/2fa/disable", authHandler.Disable2FA)

			// GitHub OAuth
			protected.GET("/auth/github/connect", githubOAuth.Connect)
			protected.GET("/auth/github/setup", githubOAuth.SetupManifest)
			protected.GET("/auth/github/status", githubOAuth.GitHubStatus)

			// Projects
			projects := v1.NewProjectHandler(deps.Services.Project)
			protected.GET("/projects", projects.List)
			protected.POST("/projects", projects.Create)

			projectGroup := protected.Group("/projects/:id")
			{
				projectGroup.GET("", projects.Get)
				projectGroup.PATCH("", projects.Update)
				projectGroup.DELETE("", projects.Delete)
				projectGroup.PUT("/env", projects.UpdateEnv)

				appHandler := v1.NewAppHandler(deps.Services.App, deps.Services.Metrics, deps.Store)
				projectGroup.GET("/apps", appHandler.ListByProject)

				dbHandler := v1.NewDatabaseHandler(deps.Services.Database, deps.Store)
				projectGroup.GET("/databases", dbHandler.ListByProject)

				cronJobHandler := v1.NewCronJobHandler(deps.Services.CronJob)
				projectGroup.GET("/cronjobs", cronJobHandler.ListByProject)
			}

			// Applications (flat)
			appHandler := v1.NewAppHandler(deps.Services.App, deps.Services.Metrics, deps.Store)
			protected.GET("/apps", appHandler.ListAll)
			protected.POST("/apps", appHandler.Create)

			// All /apps/:id routes require org ownership verification
			appByID := protected.Group("/apps/:id")
			appByID.Use(appHandler.AppOrgGuard())
			{
				appByID.GET("", appHandler.Get)
				appByID.DELETE("", appHandler.Delete)
				appByID.POST("/scale", appHandler.Scale)
				appByID.PUT("/env", appHandler.UpdateEnv)
				appByID.GET("/status", appHandler.GetStatus)
				appByID.GET("/pods", appHandler.GetPods)
				appByID.GET("/metrics", appHandler.GetMetrics)
				appByID.POST("/restart", appHandler.Restart)
				appByID.POST("/stop", appHandler.Stop)
				appByID.POST("/clear-cache", appHandler.ClearBuildCache)
				appByID.PATCH("", appHandler.Update)
				appByID.GET("/pods/:podName/events", appHandler.GetPodEvents)
				appByID.GET("/webhook", appHandler.GetWebhookConfig)
				appByID.POST("/webhook/enable", appHandler.EnableWebhook)
				appByID.POST("/webhook/disable", appHandler.DisableWebhook)
				appByID.POST("/webhook/regenerate", appHandler.RegenerateWebhook)
				appByID.GET("/secrets", appHandler.GetSecrets)
				appByID.PUT("/secrets", appHandler.UpdateSecrets)
			}
			// Deployments under apps
			deploys := v1.NewDeployHandler(deps.Services.Deploy, deps.Store)
			protected.POST("/apps/:id/deploy", deploys.Trigger)
			protected.GET("/apps/:id/deployments", deploys.List)

			// Domains under apps
			domains := v1.NewDomainHandler(deps.Services.Domain, deps.Store)
			protected.GET("/apps/:id/domains", domains.ListByApp)
			protected.POST("/apps/:id/domains", domains.Create)
			protected.POST("/apps/:id/domains/generate", domains.Generate)

			// Deployments (flat)
			protected.GET("/deployments", deploys.ListAll)
			protected.GET("/deployments/queue", deploys.ListQueue)
			protected.GET("/deployments/:id", deploys.Get)
			protected.POST("/deployments/:id/cancel", deploys.Cancel)
			protected.POST("/deployments/:id/rollback", deploys.Rollback)

			// CronJobs (flat)
			cronJobs := v1.NewCronJobHandler(deps.Services.CronJob)
			protected.POST("/cronjobs", cronJobs.Create)
			protected.GET("/cronjobs/:id", cronJobs.Get)
			protected.PATCH("/cronjobs/:id", cronJobs.Update)
			protected.DELETE("/cronjobs/:id", cronJobs.Delete)
			protected.POST("/cronjobs/:id/trigger", cronJobs.Trigger)
			protected.GET("/cronjobs/:id/runs", cronJobs.ListRuns)

			// Domains (flat - for delete and update)
			protected.DELETE("/domains/:id", domains.Delete)
			protected.PATCH("/domains/:id", domains.Update)

			// Databases (flat)
			dbHandler := v1.NewDatabaseHandler(deps.Services.Database, deps.Store)
			protected.GET("/databases/versions", dbHandler.ListVersions)
			protected.GET("/databases/used-ports", dbHandler.UsedPorts)
			protected.POST("/databases", dbHandler.Create)
			protected.GET("/databases/:id", dbHandler.Get)
			protected.DELETE("/databases/:id", dbHandler.Delete)
			protected.GET("/databases/:id/credentials", dbHandler.GetCredentials)
			protected.GET("/databases/:id/status", dbHandler.GetStatus)
			protected.GET("/databases/:id/pods", dbHandler.GetPods)
			protected.POST("/databases/:id/backups", dbHandler.TriggerBackup)
			protected.GET("/databases/:id/backups", dbHandler.ListBackups)
			protected.POST("/databases/:id/backups/:backupId/restore", dbHandler.RestoreBackup)
			protected.POST("/databases/:id/external-access", dbHandler.UpdateExternalAccess)
			protected.PUT("/databases/:id/backup-config", dbHandler.UpdateBackupConfig)

			// Templates
			templates := v1.NewTemplateHandler(deps.Services.Template)
			protected.GET("/templates", templates.List)
			protected.GET("/templates/:id", templates.Get)

			// Cluster
			cluster := v1.NewClusterHandler(deps.Orch, deps.Store)
			protected.GET("/cluster/nodes", cluster.GetNodes)
			protected.GET("/cluster/metrics", cluster.GetMetrics)
			protected.GET("/cluster/pods", cluster.GetAllPods)
			protected.GET("/cluster/events", cluster.GetEvents)
			protected.GET("/cluster/pvcs", cluster.GetPVCs)
			protected.GET("/cluster/namespaces", cluster.GetNamespaces)
			protected.GET("/cluster/node-metrics", cluster.GetNodeMetrics)
			protected.GET("/cluster/topology", cluster.GetTopology)
			protected.GET("/cluster/node-pools", cluster.GetNodePools)
			protected.PUT("/cluster/nodes/:name/pool", cluster.SetNodePool)
			protected.GET("/cluster/traefik-config", cluster.GetTraefikConfig)
			protected.PUT("/cluster/traefik-config", cluster.UpdateTraefikConfig)
			protected.POST("/cluster/traefik-restart", cluster.RestartTraefik)
			protected.GET("/cluster/traefik-status", cluster.GetTraefikStatus)
			protected.GET("/cluster/helm-releases", cluster.GetHelmReleases)
			protected.GET("/cluster/daemonsets", cluster.GetDaemonSets)
			protected.DELETE("/cluster/pvcs/:namespace/:name", cluster.DeletePVC)
			protected.PUT("/cluster/pvcs/:namespace/:name/expand", cluster.ExpandPVC)
			protected.GET("/cluster/cleanup/stats", cluster.GetCleanupStats)
			protected.POST("/cluster/cleanup/evicted-pods", cluster.CleanupEvictedPods)
			protected.POST("/cluster/cleanup/failed-pods", cluster.CleanupFailedPods)
			protected.POST("/cluster/cleanup/completed-pods", cluster.CleanupCompletedPods)
			protected.POST("/cluster/cleanup/stale-replicasets", cluster.CleanupStaleReplicaSets)
			protected.POST("/cluster/cleanup/completed-jobs", cluster.CleanupCompletedJobs)
			protected.POST("/cluster/cleanup/orphan-ingresses", cluster.CleanupOrphanIngresses)

			// Monitoring
			monitoring := v1.NewMonitoringHandler(deps.Services.Metrics)
			protected.GET("/monitoring/snapshots", monitoring.GetSnapshots)
			protected.GET("/monitoring/events", monitoring.GetEvents)
			protected.GET("/monitoring/alerts", monitoring.GetAlerts)
			protected.GET("/monitoring/alerts/active", monitoring.GetActiveAlerts)
			protected.POST("/monitoring/alerts/:id/resolve", monitoring.ResolveAlert)

			// Settings
			settings := v1.NewSettingHandler(deps.Services.Setting)
			protected.GET("/settings", settings.GetAll)
			protected.PUT("/settings", settings.Update)
			protected.GET("/settings/verify-domain", settings.VerifyDomain)

			// Notifications
			notif := v1.NewNotificationHandler(deps.Services.Notification)
			protected.GET("/notifications/channels", notif.ListChannels)
			protected.PUT("/notifications/channels", notif.SaveChannel)
			protected.POST("/notifications/test", notif.TestChannel)
			protected.GET("/settings/smtp", notif.GetSMTPConfig)
			protected.PUT("/settings/smtp", notif.SaveSMTPConfig)
			protected.POST("/settings/smtp/test", notif.TestSMTP)

			// Nodes
			nodeHandler := v1.NewNodeHandler(deps.Services.Node)
			protected.GET("/nodes", nodeHandler.List)
			protected.POST("/nodes", nodeHandler.Create)
			protected.POST("/nodes/:id/initialize", nodeHandler.Initialize)
			protected.DELETE("/nodes/:id", nodeHandler.Delete)

			// Team management
			team := v1.NewTeamHandler(deps.Services.Team, deps.Services.Notification, deps.AppURL)
			protected.GET("/team/members", team.ListMembers)
			protected.PATCH("/team/members/:id/role", middleware.RequireRole("owner"), team.UpdateMemberRole)
			protected.DELETE("/team/members/:id", middleware.RequireRole("owner"), team.RemoveMember)
			protected.POST("/team/invitations", middleware.RequireRole("owner"), team.InviteMember)
			protected.GET("/team/invitations", middleware.RequireRole("owner"), team.ListInvitations)
			protected.DELETE("/team/invitations/:id", middleware.RequireRole("owner"), team.CancelInvitation)
			protected.POST("/team/invitations/accept", team.AcceptInvitation)
			protected.PUT("/team/projects/:projectId/members/:userId", middleware.RequireRole("owner"), team.SetProjectAccess)
			protected.DELETE("/team/projects/:projectId/members/:userId", middleware.RequireRole("owner"), team.RemoveProjectAccess)

			// Version & Upgrade
			protected.GET("/version", func(c *gin.Context) {
				c.JSON(200, deps.Services.Version.GetVersionInfo(c.Request.Context()))
			})
			protected.POST("/system/upgrade", middleware.RequireRole("owner"), func(c *gin.Context) {
				if err := deps.Services.Version.TriggerUpgrade(); err != nil {
					c.JSON(400, gin.H{"detail": err.Error()})
					return
				}
				c.JSON(200, gin.H{"message": "upgrade started"})
			})
			protected.GET("/system/upgrade/status", middleware.RequireRole("owner"), func(c *gin.Context) {
				c.JSON(200, deps.Services.Version.GetUpgradeStatus())
			})
			protected.DELETE("/system/upgrade/status", middleware.RequireRole("owner"), func(c *gin.Context) {
				deps.Services.Version.ClearUpgradeStatus()
				c.JSON(200, gin.H{"message": "upgrade status cleared"})
			})

			// System Backups
			sysBackup := v1.NewSystemBackupHandler(deps.Services.SystemBackup)
			protected.GET("/system/backup/config", sysBackup.GetConfig)
			protected.PUT("/system/backup/config", sysBackup.SaveConfig)
			protected.POST("/system/backup/trigger", sysBackup.TriggerBackup)
			protected.GET("/system/backup/list", sysBackup.ListBackups)

			// Shared Resources
			resourceHandler := v1.NewResourceHandler(deps.Services.Resource)
			protected.GET("/resources", resourceHandler.List)
			protected.POST("/resources", resourceHandler.Create)
			protected.POST("/resources/generate-ssh-key", resourceHandler.GenerateSSHKey)
			protected.GET("/resources/:id/repos", resourceHandler.ListRepos)
			protected.PATCH("/resources/:id", resourceHandler.Update)
			protected.DELETE("/resources/:id", resourceHandler.Delete)
			protected.POST("/resources/:id/test", resourceHandler.TestConnection)
		}
	}

	return r
}
