package admin

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"sync"

	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"

	"github.com/spaceuptech/space-cloud/gateway/config"
	"github.com/spaceuptech/space-cloud/gateway/model"
	"github.com/spaceuptech/space-cloud/gateway/utils"
)

const maxLicenseFetchErrorCount = 5

// Manager manages all admin transactions
type Manager struct {
	lock   sync.RWMutex
	config *config.Admin
	quotas model.UsageQuotas
	plan   string
	user   *config.AdminUser
	isProd bool

	syncMan model.SyncManAdminInterface

	clusterID              string
	licenseFetchErrorCount int
	// Config for enterprise
	nodeID    string
	sessionID string
	publicKey *rsa.PublicKey
}

// New creates a new admin manager instance
func New(nodeID, clusterID string, isDev bool, adminUserInfo *config.AdminUser) *Manager {
	m := new(Manager)
	m.nodeID = nodeID
	m.isProd = !isDev // set inverted
	m.clusterID = clusterID
	if m.checkIfLeaderGateway() {
		m.sessionID = ksuid.New().String()
	}
	// Initialise all config
	m.config = new(config.Admin)
	m.user = adminUserInfo
	m.quotas = model.UsageQuotas{MaxDatabases: 1, MaxProjects: 1}

	// Start the background routines
	go m.licenseRenewalRoutine()

	return m
}

func (m *Manager) startOperation(license string, isInitialCall bool) error {
	logrus.Infoln("Starting gateway in enterprise mode")

	// Fetch the public key
	if err := m.fetchPublicKeyWithoutLock(); err != nil {
		return utils.LogError("Unable to fetch public key", "admin", "startOperation", err)
	}

	// Parse the license
	licenseObj, err := m.decryptLicense(license)
	if err != nil {
		return utils.LogError("Unable to decrypt license key", "admin", "startOperation", err)
	}

	// We have a problem if our session id does not match with the license's session id
	if m.sessionID != licenseObj.SessionID {

		// There cannot be a mismatch unless the gateway just started. For anytime else, throw an error.
		if !isInitialCall {

			// Reset quotas and admin config to defaults
			_ = utils.LogError("Invalid license file provided. Did you change the license key yourself?", "admin", "startOperation", errors.New("session id mismatch while setting admin config"))
			m.resetQuotas()
			return nil
		}

		// Renew the license to update the license session id with the current id
		if err := m.renewLicenseWithoutLock(true); err != nil {
			return err
		}
		return nil
	}

	// set quotas
	m.quotas.MaxProjects = licenseObj.Quotas.MaxProjects
	m.quotas.MaxDatabases = licenseObj.Quotas.MaxDatabases
	m.plan = licenseObj.Plan

	return nil
}

func (m *Manager) SetSyncMan(s model.SyncManAdminInterface) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.syncMan = s
}

// SetConfig sets the admin config
func (m *Manager) SetConfig(config *config.Admin, isInitialCall bool) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Set the admin config
	m.config = config

	// Check if the cluster is registered
	if m.isRegistered() {
		if m.checkIfLeaderGateway() {
			// Only the leader gateway can handle licensing information
			return m.startOperation(config.License, isInitialCall)
		} else {
			if !isInitialCall {
				// The followers will attempt to ping the leader. If ping fails they will reset the license.
				utils.LogDebug("Pinging the leader now.", "admin", "SetConfig", nil)
				if err := m.syncMan.PingLeader(); err != nil {
					_ = utils.LogError("Unable to ping the leader now.", "admin", "SetConfig", err)
					m.resetQuotas()
					return err
				}
				utils.LogDebug("Successfully contacted the leader.", "admin", "SetConfig", nil)
			}
			return m.setQuotas(config.License)
		}
	}

	utils.LogInfo("Gateway running in open source mode", "admin", "SetConfig")
	// Reset quotas defaults
	m.quotas.MaxProjects = 1
	m.quotas.MaxDatabases = 1
	m.plan = "space-cloud-open--monthly"
	return nil
}

func (m *Manager) GetConfig() *config.Admin {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.config
}

// SetEnv sets the env
func (m *Manager) SetEnv(isProd bool) {
	m.lock.Lock()
	m.isProd = isProd
	m.lock.Unlock()
}

// LoadEnv gets the env
func (m *Manager) LoadEnv() (bool, string, model.UsageQuotas) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.isProd, m.plan, m.quotas
}

// Login handles the admin login operation
func (m *Manager) Login(user, pass string) (int, string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m.user.User == user && m.user.Pass == pass {
		token, err := m.createToken(map[string]interface{}{"id": user, "role": user})
		if err != nil {
			return http.StatusInternalServerError, "", err
		}
		return http.StatusOK, token, nil
	}

	return http.StatusUnauthorized, "", errors.New("Invalid credentials provided")
}