package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/ovh/cds/engine/api/application"
	"github.com/ovh/cds/engine/api/environment"
	"github.com/ovh/cds/engine/api/notification"
	"github.com/ovh/cds/engine/api/pipeline"
	"github.com/ovh/cds/engine/api/project"
	test "github.com/ovh/cds/engine/api/testwithdb"
	"github.com/ovh/cds/sdk"
)

func deleteAll(t *testing.T, db *sql.DB, key string) error {
	// Delete all apps
	t.Logf("start deleted : %s", key)
	proj, err := project.LoadProject(db, key, &sdk.User{Admin: true})
	if err != nil {
		return err
	}

	apps, err := application.LoadApplications(db, key, false, &sdk.User{Admin: true})
	if err != nil {
		t.Logf("Cannot list app: %s", err)
		return err
	}
	for _, app := range apps {
		tx, _ := db.Begin()
		err = application.DeleteApplication(tx, app.ID)
		if err != nil {
			t.Logf("DeleteApplication: %s", err)
			return err
		}
		_ = tx.Commit()
	}

	// Delete all pipelines
	pips, err := pipeline.LoadPipelines(db, proj.ID, false, &sdk.User{Admin: true})
	if err != nil {
		t.Logf("ListPipelines: %s", err)
		return err
	}
	for _, pip := range pips {
		err = pipeline.DeletePipeline(db, pip.ID, 1)
		if err != nil {
			t.Logf("DeletePipeline: %s", err)
			return err
		}
	}

	// Delete project
	err = project.DeleteProject(db, key)
	if err != nil {
		t.Logf("RemoveProject: %s", err)
		return err
	}
	t.Logf("All deleted")
	return nil
}
func testApplicationPipelineNotifBoilerPlate(t *testing.T, f func(*testing.T, *sql.DB, *sdk.Project, *sdk.Pipeline, *sdk.Application, *sdk.Environment)) {
	if test.DBDriver == "" {
		t.SkipNow()
		return
	}
	db, err := test.SetupPG(t)
	assert.NoError(t, err)

	_ = deleteAll(t, db, "TEST_APP_PIPELINE_NOTIF")

	//Insert Project
	proj, err := test.InsertTestProject(t, db, "TEST_APP_PIPELINE_NOTIF", "TEST_APP_PIPELINE_NOTIF")
	assert.NoError(t, err)

	//Insert Pipeline
	pip := &sdk.Pipeline{
		Name:       "TEST_PIPELINE",
		Type:       sdk.BuildPipeline,
		ProjectKey: proj.Key,
		ProjectID:  proj.ID,
	}
	t.Logf("Insert Pipeline %s for Project %s", pip.Name, proj.Name)
	err = pipeline.InsertPipeline(db, pip)
	assert.NoError(t, err)

	//Insert Application
	app := &sdk.Application{
		Name: "TEST_APP",
	}
	t.Logf("Insert Application %s for Project %s", app.Name, proj.Name)
	err = application.InsertApplication(db, proj, app)

	env := &sdk.DefaultEnv

	t.Logf("Attach Pipeline %s on Application %s", pip.Name, app.Name)
	err = application.AttachPipeline(db, app.ID, pip.ID)
	assert.NoError(t, err)

	f(t, db, proj, pip, app, env)

	t.Logf("Detach Pipeline %s on Application %s", pip.Name, app.Name)
	tx, err := db.Begin()
	assert.NoError(t, err)
	err = application.RemovePipeline(tx, proj.Key, app.Name, pip.Name)
	assert.NoError(t, err)
	err = tx.Commit()
	assert.NoError(t, err)

	err = application.DeleteAllApplicationPipeline(db, app.ID)
	assert.NoError(t, err)

	err = environment.DeleteAllEnvironment(db, proj.ID)
	assert.NoError(t, err)

	//Delete application
	t.Logf("Delete Application %s for Project %s", app.Name, proj.Name)
	tx, err = db.Begin()
	assert.NoError(t, err)
	err = application.DeleteApplication(tx, app.ID)
	assert.NoError(t, err)
	err = tx.Commit()
	assert.NoError(t, err)

	//Delete pipeline
	t.Logf("Delete Pipeline %s for Project %s", pip.Name, proj.Name)
	err = pipeline.DeletePipeline(db, pip.ID, 1)
	assert.NoError(t, err)

	//Delete Project
	err = test.DeleteTestProject(t, db, "TEST_APP_PIPELINE_NOTIF")
	assert.NoError(t, err)
}

func testCheckUserNotificationSettings(t *testing.T, n1, n2 map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings) {
	for k, v := range n1 {
		t.Logf("Checkin %s: %s", k, v)
		assert.NotNil(t, n2[k])
		if k == sdk.JabberUserNotification || k == sdk.EmailUserNotification {
			j1, ok := v.(*sdk.JabberEmailUserNotificationSettings)
			assert.True(t, ok, "Should be type JabberEmailUserNotificationSettings")
			j2, ok := n2[k].(*sdk.JabberEmailUserNotificationSettings)
			assert.True(t, ok, "Should be type JabberEmailUserNotificationSettings")
			assert.Equal(t, j1.OnFailure, j2.OnFailure)
			assert.Equal(t, j1.OnSuccess, j2.OnSuccess)
			assert.Equal(t, j1.OnStart, j2.OnStart)
			assert.Equal(t, j1.SendToAuthor, j2.SendToAuthor)
			assert.Equal(t, j1.SendToGroups, j2.SendToGroups)
			assert.Equal(t, len(j1.Recipients), len(j2.Recipients))
			if len(j1.Recipients) == len(j2.Recipients) {
				for i := range j1.Recipients {
					assert.Equal(t, j1.Recipients[i], j2.Recipients[i])
				}
			}
			assert.Equal(t, j1.Template.Subject, j2.Template.Subject)
			assert.Equal(t, j1.Template.Body, j2.Template.Body)
		}
	}
}

func Test_LoadEmptyApplicationPipelineNotif(t *testing.T) {
	testApplicationPipelineNotifBoilerPlate(t, func(t *testing.T, db *sql.DB, proj *sdk.Project, pip *sdk.Pipeline, app *sdk.Application, env *sdk.Environment) {
		t.Logf("Load Application Pipeline Notif %s %s", app.Name, env.Name)
		notif, err := notification.LoadUserNotificationSettings(db, app.ID, pip.ID, env.ID)
		assert.NoError(t, err)
		assert.Nil(t, notif)
	})
}

func Test_InsertAndLoadApplicationPipelineNotif(t *testing.T) {
	testApplicationPipelineNotifBoilerPlate(t, func(t *testing.T, db *sql.DB, proj *sdk.Project, pip *sdk.Pipeline, app *sdk.Application, env *sdk.Environment) {
		notif := sdk.UserNotification{
			Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
				sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success",
					OnStart:      true,
					OnFailure:    "on_failure",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject",
						Body:    "body",
					},
				},
				sdk.EmailUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success_",
					OnStart:      true,
					OnFailure:    "on_failure_",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2", "3"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject_",
						Body:    "body_",
					},
				},
				sdk.TATUserNotification: &sdk.TATUserNotificationSettings{
					OnSuccess: "on_success__",
					OnStart:   true,
					OnFailure: "on_failure__",
					Topics:    []string{"1", "2"},
					Template:  "template",
				},
			},
			Environment: *env,
		}

		err := notification.InsertOrUpdateUserNotificationSettings(db, app.ID, pip.ID, env.ID, &notif)
		assert.NoError(t, err)

		t.Logf("Load Application Pipeline Notif %s %s", app.Name, env.Name)
		notif1, err := notification.LoadUserNotificationSettings(db, app.ID, pip.ID, env.ID)
		assert.NoError(t, err)
		assert.NotNil(t, notif1)

		testCheckUserNotificationSettings(t, notif.Notifications, notif1.Notifications)
	})
}

func Test_getUserNotificationApplicationPipelineHandlerReturnsEmptyUserNotification(t *testing.T) {
	testApplicationPipelineNotifBoilerPlate(t, func(t *testing.T, db *sql.DB, proj *sdk.Project, pip *sdk.Pipeline, app *sdk.Application, env *sdk.Environment) {
		url := fmt.Sprintf("/test1/project/%s/application/%s/pipeline/%s/notification", proj.Key, app.Name, pip.Name)
		req, err := http.NewRequest("GET", url, nil)

		router := mux.NewRouter()
		router.HandleFunc("/test1/project/{key}/application/{permApplicationName}/pipeline/{permPipelineKey}/notification",
			func(w http.ResponseWriter, r *http.Request) {
				getUserNotificationApplicationPipelineHandler(w, r, db, nil)
			})
		http.Handle("/test1/", router)

		assert.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 204, w.Code)
		assert.Equal(t, "null", w.Body.String())
	})
}

func Test_getUserNotificationApplicationPipelineHandlerReturnsNonEmptyUserNotification(t *testing.T) {
	testApplicationPipelineNotifBoilerPlate(t, func(t *testing.T, db *sql.DB, proj *sdk.Project, pip *sdk.Pipeline, app *sdk.Application, env *sdk.Environment) {
		notif := sdk.UserNotification{
			Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
				sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success",
					OnStart:      true,
					OnFailure:    "on_failure",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject",
						Body:    "body",
					},
				},
				sdk.EmailUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success_",
					OnStart:      true,
					OnFailure:    "on_failure_",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2", "3"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject_",
						Body:    "body_",
					},
				},
				sdk.TATUserNotification: &sdk.TATUserNotificationSettings{
					OnSuccess: "on_success__",
					OnStart:   true,
					OnFailure: "on_failure__",
					Topics:    []string{"1", "2"},
					Template:  "template",
				},
			},
		}

		err := notification.InsertOrUpdateUserNotificationSettings(db, app.ID, pip.ID, env.ID, &notif)
		assert.NoError(t, err)

		url := fmt.Sprintf("/test2/project/%s/application/%s/pipeline/%s/notification", proj.Key, app.Name, pip.Name)
		req, err := http.NewRequest("GET", url, nil)

		router := mux.NewRouter()
		router.HandleFunc("/test2/project/{key}/application/{permApplicationName}/pipeline/{permPipelineKey}/notification",
			func(w http.ResponseWriter, r *http.Request) {
				getUserNotificationApplicationPipelineHandler(w, r, db, nil)
			})
		http.Handle("/test2/", router)

		assert.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		notif1, err := notification.ParseUserNotification(w.Body.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, notif.ApplicationPipelineID, notif1.ApplicationPipelineID)
		assert.Equal(t, notif.Environment.ID, notif1.Environment.ID)

		testCheckUserNotificationSettings(t, notif.Notifications, notif1.Notifications)
	})
}

func Test_getNotificationTypeHandler(t *testing.T) {
	url := fmt.Sprintf("/test3/notification/type")
	req, err := http.NewRequest("GET", url, nil)

	router := mux.NewRouter()
	router.HandleFunc("/test3/notification/type",
		func(w http.ResponseWriter, r *http.Request) {
			getUserNotificationTypeHandler(w, r, nil, nil)
		})
	http.Handle("/test3/", router)

	assert.NoError(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var s = []string{}
	err = json.Unmarshal(w.Body.Bytes(), &s)
	assert.NoError(t, err)
	assert.Equal(t, 200, w.Code)
}

func Test_updateUserNotificationApplicationPipelineHandler(t *testing.T) {
	testApplicationPipelineNotifBoilerPlate(t, func(t *testing.T, db *sql.DB, proj *sdk.Project, pip *sdk.Pipeline, app *sdk.Application, env *sdk.Environment) {
		notif := sdk.UserNotification{
			Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
				sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success",
					OnStart:      true,
					OnFailure:    "on_failure",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject",
						Body:    "body",
					},
				},
				sdk.EmailUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success_",
					OnStart:      true,
					OnFailure:    "on_failure_",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2", "3"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject_",
						Body:    "body_",
					},
				},
				sdk.TATUserNotification: &sdk.TATUserNotificationSettings{
					OnSuccess: "on_success__",
					OnStart:   true,
					OnFailure: "on_failure__",
					Topics:    []string{"1", "2"},
					Template:  "template",
				},
			},
		}

		err := notification.InsertOrUpdateUserNotificationSettings(db, app.ID, pip.ID, env.ID, &notif)
		assert.NoError(t, err)

		notif = sdk.UserNotification{
			Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
				sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success",
					OnStart:      true,
					OnFailure:    "on_failure",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2"},
					Template: sdk.UserNotificationTemplate{
						Subject: "subject",
						Body:    "body",
					},
				},
			},
			Environment: *env,
		}

		b, err := json.Marshal(notif)
		assert.NoError(t, err)
		body := bytes.NewBuffer(b)

		url := fmt.Sprintf("/test4/project/%s/application/%s/pipeline/%s/notification", proj.Key, app.Name, pip.Name)
		req, err := http.NewRequest("POST", url, body)
		assert.NoError(t, err)

		router := mux.NewRouter()
		router.HandleFunc("/test4/project/{key}/application/{permApplicationName}/pipeline/{permPipelineKey}/notification",
			func(w http.ResponseWriter, r *http.Request) {
				updateUserNotificationApplicationPipelineHandler(w, r, db, nil)
			})

		http.Handle("/test4/", router)

		assert.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)

		notif1, err := notification.ParseUserNotification(w.Body.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, notif.ApplicationPipelineID, notif1.ApplicationPipelineID)
		assert.Equal(t, notif.Environment.ID, notif1.Environment.ID)

		testCheckUserNotificationSettings(t, notif.Notifications, notif1.Notifications)

	})
}

func Test_ShouldSendUserNotificationOnStartTrue(t *testing.T) {
	notif := sdk.UserNotification{
		Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
			sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
				OnSuccess:    "always",
				OnStart:      true,
				OnFailure:    "always",
				SendToAuthor: true,
				SendToGroups: true,
				Recipients:   []string{"1", "2"},
				Template: sdk.UserNotificationTemplate{
					Subject: "subject",
					Body:    "body",
				},
			},
		},
	}

	current := sdk.PipelineBuild{
		Status: sdk.StatusBuilding,
	}

	assert.True(t, notification.ShouldSendUserNotification(notif.Notifications[sdk.JabberUserNotification], &current, nil))
}

func Test_ShouldNotSendUserNotificationOnStartFalse(t *testing.T) {
	notif := sdk.UserNotification{
		Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
			sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
				OnSuccess:    "always",
				OnStart:      false,
				OnFailure:    "always",
				SendToAuthor: true,
				SendToGroups: true,
				Recipients:   []string{"1", "2"},
				Template: sdk.UserNotificationTemplate{
					Subject: "subject",
					Body:    "body",
				},
			},
		},
	}

	current := sdk.PipelineBuild{
		Status: sdk.StatusBuilding,
	}

	assert.False(t, notification.ShouldSendUserNotification(notif.Notifications[sdk.JabberUserNotification], &current, nil))
}

func Test_ShouldSendUserNotificationOnSuccessAlways(t *testing.T) {
	notif := sdk.UserNotification{
		Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
			sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
				OnSuccess:    "always",
				OnStart:      true,
				OnFailure:    "always",
				SendToAuthor: true,
				SendToGroups: true,
				Recipients:   []string{"1", "2"},
				Template: sdk.UserNotificationTemplate{
					Subject: "subject",
					Body:    "body",
				},
			},
		},
	}

	current := sdk.PipelineBuild{
		Status: sdk.StatusSuccess,
	}

	assert.True(t, notification.ShouldSendUserNotification(notif.Notifications[sdk.JabberUserNotification], &current, nil))
}

func Test_ShouldNotSendUserNotificationOnSuccessNever(t *testing.T) {
	notif := sdk.UserNotification{
		Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
			sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
				OnSuccess:    "never",
				OnStart:      true,
				OnFailure:    "always",
				SendToAuthor: true,
				SendToGroups: true,
				Recipients:   []string{"1", "2"},
				Template: sdk.UserNotificationTemplate{
					Subject: "subject",
					Body:    "body",
				},
			},
		},
	}

	current := sdk.PipelineBuild{
		Status: sdk.StatusSuccess,
	}

	assert.False(t, notification.ShouldSendUserNotification(notif.Notifications[sdk.JabberUserNotification], &current, nil))
}

func Test_SendPipeline(t *testing.T) {
	testApplicationPipelineNotifBoilerPlate(t, func(t *testing.T, db *sql.DB, proj *sdk.Project, pip *sdk.Pipeline, app *sdk.Application, env *sdk.Environment) {
		notif := sdk.UserNotification{
			Notifications: map[sdk.UserNotificationSettingsType]sdk.UserNotificationSettings{
				sdk.JabberUserNotification: &sdk.JabberEmailUserNotificationSettings{
					OnSuccess:    "on_success",
					OnStart:      true,
					OnFailure:    "on_failure",
					SendToAuthor: true,
					SendToGroups: true,
					Recipients:   []string{"1", "2"},
					Template: sdk.UserNotificationTemplate{
						Subject: "CDS {{.cds.project}}/{{.cds.application}} {{.cds.pipeline}} {{.cds.status}}",
						Body:    "\nDetails : {{.cds.buildURL}}",
					},
				},
			},
		}
		err := notification.InsertOrUpdateUserNotificationSettings(db, app.ID, pip.ID, env.ID, &notif)
		assert.NoError(t, err)

		tx, err := db.Begin()
		assert.NoError(t, err)

		params := []sdk.Parameter{}
		trigger := sdk.PipelineBuildTrigger{}

		//mock cds2xmpp server
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/jabber/build" {
					decoder := json.NewDecoder(r.Body)
					var n sdk.Notif
					err = decoder.Decode(&n)
					assert.NoError(t, err)
					assert.Equal(t, "CDS TEST_APP_PIPELINE_NOTIF/TEST_APP TEST_PIPELINE Building", n.Title)
					assert.Equal(t, "\nDetails : http://localhost:9000/#/project/TEST_APP_PIPELINE_NOTIF/application/TEST_APP/pipeline/TEST_PIPELINE/build/1?env=NoEnv&tab=detail", n.Message)
				}
				w.WriteHeader(200)
			},
		))
		defer server.Close()

		//Initialize notification sender...
		notification.Initialize("jabber:"+server.URL, "jabber", "http://localhost:9000")

		t.Log("Insert PipelineBuild")

		pb, err := pipeline.InsertPipelineBuild(tx, proj, pip, app, params, params, env, -1, trigger)
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		err = pipeline.DeletePipelineBuild(db, pb.ID)
		assert.NoError(t, err)

	})
}