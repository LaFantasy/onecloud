// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/tristate"
	"yunion.io/x/pkg/util/sets"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/apis"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	cop "yunion.io/x/onecloud/pkg/compute/options"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/mcclient/options"
	"yunion.io/x/onecloud/pkg/util/httputils"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

var ScheduledTaskManager *SScheduledTaskManager

func init() {
	ScheduledTaskManager = &SScheduledTaskManager{
		SVirtualResourceBaseManager: db.NewVirtualResourceBaseManager(
			SScheduledTask{},
			"scheduledtasks_tbl",
			"scheduledtask",
			"scheduledtasks",
		),
	}
	ScheduledTaskManager.SetVirtualObject(ScheduledTaskManager)
}

// +onecloud:swagger-gen-model-singular=scheduledtask
// +onecloud:swagger-gen-model-singular=scheduledtasks
type SScheduledTaskManager struct {
	db.SVirtualResourceBaseManager
	db.SEnabledResourceBaseManager
}

type SScheduledTask struct {
	db.SVirtualResourceBase
	db.SEnabledResourceBase

	ScheduledType string `width:"16" charset:"ascii" create:"required" list:"user" get:"user"`

	STimer

	TimerDesc    string `width:"128" charset:"utf8" list:"user" get:"user"`
	ResourceType string `width:"32" charset:"ascii" create:"required" list:"user" get:"user"`
	Operation    string `width:"32" charset:"ascii" create:"required" list:"user" get:"user"`
	LabelType    string `width:"4" charset:"ascii" create:"required" list:"user" get:"user"`
}

func (stm *SScheduledTaskManager) ListItemFilter(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, input api.ScheduledTaskListInput) (*sqlchemy.SQuery, error) {
	var err error
	q, err = stm.SVirtualResourceBaseManager.ListItemFilter(ctx, q, userCred, input.VirtualResourceListInput)
	if err != nil {
		return q, err
	}
	q, err = stm.SEnabledResourceBaseManager.ListItemFilter(ctx, q, userCred, input.EnabledResourceBaseListInput)
	if err != nil {
		return q, err
	}
	if len(input.Operation) > 0 {
		q = q.Equals("operation", input.Operation)
	}
	if len(input.ResourceType) > 0 {
		q = q.Equals("resource_type", input.ResourceType)
	}
	if len(input.LabelType) > 0 {
		q = q.Equals("label_type", input.LabelType)
	}
	return q, nil
}

func (stm *SScheduledTaskManager) OrderByExtraFields(ctx context.Context, q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential, query api.ScalingPolicyListInput) (*sqlchemy.SQuery, error) {
	return stm.SVirtualResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.VirtualResourceListInput)
}

func (stm *SScheduledTask) GetExtraDetails(ctx context.Context, userCred mcclient.TokenCredential,
	query jsonutils.JSONObject, isList bool) (api.ScheduledTaskDetails, error) {
	return api.ScheduledTaskDetails{}, nil
}

func (stm *SScheduledTaskManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.ScheduledTaskDetails {
	rows := make([]api.ScheduledTaskDetails, len(objs))
	virRows := stm.SVirtualResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	var err error
	for i := range rows {
		rows[i], err = objs[i].(*SScheduledTask).getMoreDetails(ctx, userCred, query, isList)
		if err != nil {
			log.Errorf("SScheduledTask.getMoreDetails error: %s", err)
		}
		rows[i].VirtualResourceDetails = virRows[i]
	}
	return rows
}

func (st *SScheduledTask) getMoreDetails(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, isList bool) (api.ScheduledTaskDetails, error) {
	var out api.ScheduledTaskDetails
	switch st.ScheduledType {
	case api.ST_TYPE_TIMING:
		out.Timer = st.STimer.TimerDetails()
	case api.ST_TYPE_CYCLE:
		out.CycleTimer = st.STimer.CycleTimerDetails()
	}
	// fill label
	stLabels, err := st.STLabels()
	if err != nil {
		return out, err
	}
	out.Labels = make([]string, len(stLabels))
	out.LabelDetails = make([]api.LabelDetail, len(stLabels))
	for i := range stLabels {
		out.Labels[i] = stLabels[i].Label
		out.LabelDetails[i].IsolatedTime = stLabels[i].CreatedAt
		out.LabelDetails[i].Label = stLabels[i].Label
	}
	return out, nil
}

func (stm *SScheduledTaskManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.ScheduledTaskCreateInput) (api.ScheduledTaskCreateInput, error) {
	var err error
	input.VirtualResourceCreateInput, err = stm.SVirtualResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.VirtualResourceCreateInput)
	if err != nil {
		return input, err
	}
	if !utils.IsInStringArray(input.ScheduledType, []string{api.ST_TYPE_TIMING, api.ST_TYPE_CYCLE}) {
		return input, httperrors.NewInputParameterError("unkown scheduled type '%s'", input.ScheduledType)
	}
	if !utils.IsInStringArray(input.ResourceType, []string{api.ST_RESOURCE_SERVER}) {
		return input, httperrors.NewInputParameterError("unkown resource type '%s'", input.ResourceType)
	}
	if !utils.IsInStringArray(input.Operation, []string{api.ST_RESOURCE_OPERATION_RESTART, api.ST_RESOURCE_OPERATION_STOP, api.ST_RESOURCE_OPERATION_START}) {
		return input, httperrors.NewInputParameterError("unkown resource operation '%s'", input.Operation)
	}
	if !utils.IsInStringArray(input.LabelType, []string{api.ST_LABEL_ID, api.ST_LABEL_TAG}) {
		return input, httperrors.NewInputParameterError("unkown label type '%s'", input.LabelType)
	}
	// check timer or cycletimer
	if input.ScheduledType == api.ST_TYPE_TIMING {
		input.Timer, err = checkTimerCreateInput(input.Timer)
	} else {
		input.CycleTimer, err = checkCycleTimerCreateInput(input.CycleTimer)
	}
	return input, err
}

var wdsCN = []string{"", "一", "二", "三", "四", "五", "六", "日"}
var zone = time.FixedZone("GMT", 8*3600)

func (st *SScheduledTask) TimerDescription() string {
	format := "2006-01-02 15:04:05"
	var prefix string
	timer := st.STimer
	switch timer.Type {
	case api.TIMER_TYPE_ONCE:
		return fmt.Sprintf("单次 %s触发", timer.StartTime.In(zone).Format(format))
	case api.TIMER_TYPE_DAY:
		prefix = "每天"
	case api.TIMER_TYPE_WEEK:
		wds := timer.GetWeekDays()
		weekDays := make([]string, len(wds))
		for i := range wds {
			weekDays[i] = fmt.Sprintf("星期%s", wdsCN[wds[i]])
		}
		prefix = fmt.Sprintf("每周 【%s】", strings.Join(weekDays, "｜"))
	case api.TIMER_TYPE_MONTH:
		mns := timer.GetMonthDays()
		monthDays := make([]string, len(mns))
		for i := range mns {
			monthDays[i] = fmt.Sprintf("%d号", mns[i])
		}
		prefix = fmt.Sprintf("每月 【%s】", strings.Join(monthDays, "｜"))
	}
	return fmt.Sprintf("%s %2d:%2d触发 有效时间为%s至%s", prefix, timer.Hour, timer.Minute, timer.StartTime.In(zone).Format(format), timer.EndTime.In(zone).Format(format))
}

func (st *SScheduledTask) AllowPerformEnable(ctx context.Context, userCred mcclient.TokenCredential,
	query jsonutils.JSONObject, input apis.PerformEnableInput) bool {
	return true
}

func (st *SScheduledTask) PerformEnable(ctx context.Context, userCred mcclient.TokenCredential,
	query jsonutils.JSONObject, input apis.PerformEnableInput) (jsonutils.JSONObject, error) {
	err := db.EnabledPerformEnable(st, ctx, userCred, true)
	if err != nil {
		return nil, errors.Wrap(err, "EnabledPerformEnable")
	}
	return nil, nil
}

func (st *SScheduledTask) AllowPerformDisable(ctx context.Context, userCred mcclient.TokenCredential,
	query jsonutils.JSONObject, input apis.PerformDisableInput) bool {
	return true
}

func (st *SScheduledTask) PerformDisable(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject,
	input apis.PerformDisableInput) (jsonutils.JSONObject, error) {
	err := db.EnabledPerformEnable(st, ctx, userCred, false)
	if err != nil {
		return nil, errors.Wrap(err, "EnabledPerformEnable")
	}
	return nil, nil
}

func (st *SScheduledTask) PostCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	st.SVirtualResourceBase.PostCreate(ctx, userCred, ownerId, query, data)
	// add label
	createFailed := func(reason string) {
		st.SetStatus(userCred, api.ST_STATUS_CREATE_FAILED, reason)
		logclient.AddActionLogWithContext(ctx, st, logclient.ACT_CREATE, reason, userCred, false)
	}
	labels, _ := data.GetArray("labels")
	for i := range labels {
		label, _ := labels[i].GetString()
		err := ScheduledTaskLabelManager.Attach(ctx, st.Id, label)
		if err != nil {
			reason := fmt.Sprintf("unable to attach scheduled task '%s' with '%s'", st.Id, label)
			createFailed(reason)
			return
		}
	}
	input := api.ScheduledTaskCreateInput{}
	err := data.Unmarshal(&input)
	if err != nil {
		createFailed(err.Error())
		return
	}
	switch st.ScheduledType {
	case api.ST_TYPE_TIMING:
		st.STimer = STimer{
			Type:      api.TIMER_TYPE_ONCE,
			StartTime: input.Timer.ExecTime,
			EndTime:   input.Timer.ExecTime,
			NextTime:  input.Timer.ExecTime,
		}
	case api.ST_TYPE_CYCLE:
		st.STimer = STimer{
			Type:      input.CycleTimer.CycleType,
			Minute:    input.CycleTimer.Minute,
			Hour:      input.CycleTimer.Hour,
			StartTime: input.CycleTimer.StartTime,
			EndTime:   input.CycleTimer.EndTime,
			NextTime:  time.Time{},
		}
		st.SetWeekDays(input.CycleTimer.WeekDays)
		st.SetMonthDays(input.CycleTimer.MonthDays)
	}
	st.Update(time.Time{})
	st.Status = api.ST_STATUS_READY
	st.Enabled = tristate.True
	st.TimerDesc = st.TimerDescription()
	err = st.GetModelManager().TableSpec().InsertOrUpdate(ctx, st)
	if err != nil {
		createFailed("update itself")
		return
	}
	logclient.AddActionLogWithContext(ctx, st, logclient.ACT_CREATE, "", userCred, true)
}

func (st *SScheduledTask) ValidateDeleteCondition(ctx context.Context) error {
	err := st.SVirtualResourceBase.ValidateDeleteCondition(ctx)
	if err != nil {
		return err
	}
	ok, err := st.IsExecuted()
	if err != nil {
		return err
	}
	if ok {
		return httperrors.NewForbiddenError("This scheduled task is being executed now, please try later")
	}
	return nil
}

func (st *SScheduledTask) IsExecuted() (bool, error) {
	q := ScheduledTaskActivityManager.Query().Equals("status", api.ST_ACTIVITY_STATUS_EXEC).Equals("scheduled_task_id", st.Id)
	n, err := q.CountWithError()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (st *SScheduledTask) Labels() ([]string, error) {
	stLabels, err := st.STLabels()
	if err != nil {
		return nil, err
	}
	labels := make([]string, len(stLabels))
	for i := range labels {
		labels[i] = stLabels[i].Label
	}
	return labels, nil
}

func (st *SScheduledTask) STLabels() ([]SScheduledTaskLabel, error) {
	q := ScheduledTaskLabelManager.Query().Equals("scheduled_task_id", st.Id)
	labels := make([]SScheduledTaskLabel, 0, 1)
	err := db.FetchModelObjects(ScheduledTaskLabelManager, q, &labels)
	return labels, err
}

func (st *SScheduledTask) AllowPerformSetLabels(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.ScheduledTaskSetLabelsInput) bool {
	return st.IsOwner(userCred) || db.IsAdminAllowPerform(userCred, st, "set-labels")
}

func (st *SScheduledTask) PerformSetLabels(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.ScheduledTaskSetLabelsInput) (jsonutils.JSONObject, error) {
	nowLabels, err := st.STLabels()
	if err != nil {
		return nil, err
	}
	nowLabelMap := make(map[string]*SScheduledTaskLabel, len(nowLabels))
	for i := range nowLabels {
		nowLabelMap[nowLabels[i].Label] = &nowLabels[i]
	}
	futureLabelSet := sets.NewString(input.Labels...)
	var attachs []string
	var detachs []*SScheduledTaskLabel
	for label := range futureLabelSet {
		if _, ok := nowLabelMap[label]; !ok {
			attachs = append(attachs, label)
		}
	}
	for label, stLable := range nowLabelMap {
		if !futureLabelSet.Has(label) {
			detachs = append(detachs, stLable)
		}
	}
	// attach
	for _, label := range attachs {
		err := ScheduledTaskLabelManager.Attach(ctx, st.Id, label)
		if err != nil {
			return nil, err
		}
	}
	// detach
	for _, stLabel := range detachs {
		err := stLabel.Detach(ctx, userCred)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (st *SScheduledTask) AllowPerformTrigger(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.ScheduledTaskTriggerInput) bool {
	return st.IsOwner(userCred) || db.IsAdminAllowPerform(userCred, st, "trigger")
}

func (st *SScheduledTask) PerformTrigger(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.ScheduledTaskTriggerInput) (jsonutils.JSONObject, error) {
	go func() {
		log.Infof("start to execute scheduled task '%s'", st.Id)
		err := st.Execute(ctx, userCred)
		if err != nil {
			log.Errorf("fail to execute scheduled task '%s': %s", st.Id, err.Error())
		} else {
			log.Infof("execute scheduled task '%s' successfully", st.Id)
		}
	}()
	return nil, nil
}

func (st *SScheduledTask) CustomizeDelete(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	err := st.SVirtualResourceBase.CustomizeDelete(ctx, userCred, query, data)
	if err != nil {
		return err
	}
	labels, err := st.STLabels()
	if err != nil {
		return err
	}
	for i := range labels {
		err := labels[i].Delete(ctx, userCred)
		if err != nil {
			log.Errorf("unbale to delete scheduled task label: %s", err.Error())
		}
	}
	return nil
}

func (st *SScheduledTask) Action(ctx context.Context, userCred mcclient.TokenCredential) SAction {
	session := auth.GetSession(ctx, userCred, "", "")
	return Action.ResourceOperation(st.ResourceOperation()).Session(session)
}

func (st *SScheduledTask) Execute(ctx context.Context, userCred mcclient.TokenCredential) (err error) {
	exec, err := st.IsExecuted()
	if err != nil {
		return errors.Wrap(err, "unable to check if scheduled task is executed")
	}
	if exec {
		_, err := st.NewActivity(ctx, true)
		return err
	}
	sa, err := st.NewActivity(ctx, false)
	if err != nil {
		return err
	}
	over := false
	defer func() {
		if !over && err != nil {
			sa.Fail(err.Error())
		}
	}()
	action := st.Action(ctx, userCred)
	// Get All Resource
	labels, err := st.Labels()
	if err != nil {
		return err
	}

	var ids []string
	switch st.LabelType {
	case api.ST_LABEL_TAG:
		f := false
		limit := 1000
		opts := options.BaseListOptions{
			Details: &f,
			Limit:   &limit,
			Scope:   "system",
			Tags:    labels,
		}
		res, err := action.List(&WrapperListOptions{opts})
		if err != nil {
			return err
		}
		for id := range res {
			ids = append(ids, id)
		}
	case api.ST_LABEL_ID:
		ids = labels
	}

	maxLimit := 20
	type result struct {
		id      string
		succeed bool
		reason  string
	}
	workerQueue := make(chan struct{}, maxLimit)
	results := make([]result, len(ids))
	for i, id := range ids {
		workerQueue <- struct{}{}
		go func(n int, id string) {
			ok, reason := action.Apply(id)
			results[n] = result{id, ok, reason}
			<-workerQueue
		}(i, id)
	}
	// wait all finish
	for i := 0; i < maxLimit; i++ {
		workerQueue <- struct{}{}
	}
	failedReasons := make([]string, 0, 1)
	succeedIds := make([]string, 0, 1)
	for _, ret := range results {
		if ret.succeed {
			succeedIds = append(succeedIds, ret.id)
			continue
		}
		failedReasons = append(failedReasons, fmt.Sprintf("\t%s: %s", ret.id, ret.reason))
	}
	if len(failedReasons) == 0 {
		sa.Succeed()
		return nil
	}
	if len(failedReasons) == len(ids) {
		reason := fmt.Sprintf("All %ss %s failed:\n%s", st.ResourceType, st.Operation, strings.Join(failedReasons, ";\n"))
		sa.Fail(reason)
		return nil
	}
	reason := fmt.Sprintf("Some %ss %s successfully:\n\t%s\n\n. Some %ss %s failed:\n%s", st.ResourceType, st.Operation, strings.Join(succeedIds, ";"), st.ResourceType, st.Operation, strings.Join(failedReasons, ";\n"))
	sa.PartFail(reason)
	return nil
}

func (st *SScheduledTask) NewActivity(ctx context.Context, reject bool) (*SScheduledTaskActivity, error) {
	now := time.Now()
	sa := &SScheduledTaskActivity{
		StartTime: now,
	}
	sa.Status = api.ST_ACTIVITY_STATUS_EXEC
	sa.ScheduledTaskId = st.Id
	if reject {
		sa.Status = api.ST_ACTIVITY_STATUS_REJECT
		sa.EndTime = now
		sa.Reason = "This Scheduled Task is being executed now"
	}
	err := ScheduledTaskActivityManager.TableSpec().Insert(ctx, sa)
	if err != nil {
		return nil, err
	}
	sa.SetModelManager(ScheduledTaskActivityManager, sa)
	return sa, nil
}

func (st *SScheduledTask) ResourceOperation() ResourceOperation {
	return ResourceOperationMap[fmt.Sprintf("%s.%s", st.ResourceType, st.Operation)]
}

type STimeScope struct {
	Start  time.Time
	End    time.Time
	Median time.Time
}

func (stm *SScheduledTaskManager) timeScope(median time.Time, interval time.Duration) STimeScope {
	ri := interval / 2
	return STimeScope{
		Start:  median.Add(-ri),
		End:    median.Add(ri),
		Median: median,
	}
}

var timerQueue = make(chan struct{}, cop.Options.ScheduledTaskQueueSize)

func (stm *SScheduledTaskManager) Timer(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	if len(timerQueue) == 0 {
		timerQueue = make(chan struct{}, cop.Options.ScheduledTaskQueueSize)
	}
	// 60 is for fault tolerance
	interval := 60 + 30
	timeScope := stm.timeScope(time.Now(), time.Duration(interval)*time.Second)
	q := stm.Query().Equals("status", api.ST_STATUS_READY).Equals("enabled", true).LT("next_time", timeScope.End).IsFalse("is_expired")
	sts := make([]SScheduledTask, 0, 5)
	err := db.FetchModelObjects(stm, q, &sts)
	if err != nil {
		log.Errorf("db.FetchModelObjects error: %s", err.Error())
		return
	}
	log.Debugf("timeScope: start: %s, end: %s", timeScope.Start, timeScope.End)
	for i := range sts {
		st := sts[i]
		timerQueue <- struct{}{}
		go func(ctx context.Context) {
			defer func() {
				<-timerQueue
			}()
			if st.NextTime.Before(timeScope.Start) {
				// For unknown reasons, the scalingTimer did not execute at the specified time
				st.Update(timeScope.Start)
				// scalingTimer should not exec for now.
				if st.NextTime.After(timeScope.End) || st.IsExpired {
					err = stm.TableSpec().InsertOrUpdate(ctx, &st)
					if err != nil {
						log.Errorf("update Scheduled task whose id is %s error: %s", st.Id, err.Error())
					}
					return
				}
			}
			err := st.Execute(ctx, userCred)
			if err != nil {
				log.Errorf("unable to execute scheduled task '%s'", st.Id)
			}
			st.Update(timeScope.End)
			err = stm.TableSpec().InsertOrUpdate(ctx, &st)
			if err != nil {
				log.Errorf("update Scheduled task whose id is %s error: %s", st.Id, err.Error())
			}
		}(ctx)
	}
	// wait all finish
}

func init() {
	Register(ResourceServer, modules.Servers.ResourceManager)
}

// Modules describe the correspondence between Resource and modulebase.ResourceManager,
// which is equivalent to onecloud resource client.
var Modules = make(map[Resource]modulebase.ResourceManager)

// Every Resource should call Register to register their modulebase.ResourceManager.
func Register(resource Resource, manager modulebase.ResourceManager) {
	Modules[resource] = manager
}

// Resoruce describe a onecloud resource, such as:
type Resource string

const (
	ResourceServer Resource = api.ST_RESOURCE_SERVER
)

// ResourceOperation describe the operation for onecloud resource like create, update, delete and so on.
type ResourceOperation struct {
	Resource      Resource
	Operation     string
	StatusSuccess []string
	Fail          []ResourceOperationFail
}

type ResourceOperationFail struct {
	Status   string
	LogEvent string
}

// It is clearer to write each ResourceOperation as a constant
var (
	ServerStart = ResourceOperation{
		Resource:      ResourceServer,
		Operation:     api.ST_RESOURCE_OPERATION_START,
		StatusSuccess: []string{api.VM_RUNNING},
		Fail: []ResourceOperationFail{
			{api.VM_START_FAILED, db.ACT_START_FAIL},
		},
	}
	ServerStop = ResourceOperation{
		Resource:      ResourceServer,
		Operation:     api.ST_RESOURCE_OPERATION_STOP,
		StatusSuccess: []string{api.VM_READY},
		Fail: []ResourceOperationFail{
			{api.VM_STOP_FAILED, db.ACT_STOP_FAIL},
		},
	}
	ServerRestart = ResourceOperation{
		Resource:      ResourceServer,
		Operation:     api.ST_RESOURCE_OPERATION_RESTART,
		StatusSuccess: []string{api.VM_RUNNING},
		Fail: []ResourceOperationFail{
			{api.VM_START_FAILED, db.ACT_START_FAIL},
			{api.VM_STOP_FAILED, db.ACT_STOP_FAIL},
		},
	}
	ResourceOperationMap = map[string]ResourceOperation{
		fmt.Sprintf("%s.%s", ResourceServer, api.ST_RESOURCE_OPERATION_START):   ServerStart,
		fmt.Sprintf("%s.%s", ResourceServer, api.ST_RESOURCE_OPERATION_STOP):    ServerStop,
		fmt.Sprintf("%s.%s", ResourceServer, api.ST_RESOURCE_OPERATION_RESTART): ServerRestart,
	}
)

// Action itself is meaningless, a meaningful Action is generated by
// calling Resource, Operation, Session and DefaultParams.
// A example:
// 	Action.ResourceOperation(ServerStart).Session(...).Apply(...)
var Action = SAction{timeout: 5 * time.Minute}

// SAction encapsulates action to for onecloud resources
type SAction struct {
	operation    ResourceOperation
	session      *mcclient.ClientSession
	defautParams *jsonutils.JSONDict
	timeout      time.Duration
}

func (r SAction) ResourceOperation(oper ResourceOperation) SAction {
	r.operation = oper
	return r
}

func (r SAction) Session(session *mcclient.ClientSession) SAction {
	r.session = session
	return r
}

func (r SAction) Timeout(time time.Duration) SAction {
	r.timeout = time
	return r
}

func (r SAction) DefaultParams(dict *jsonutils.JSONDict) SAction {
	r.defautParams = dict
	return r
}

type WrapperListOptions struct {
	options.BaseListOptions
}

func (r SAction) List(opts *WrapperListOptions) (map[string]string, error) {
	resourceManager, ok := Modules[r.operation.Resource]
	if !ok {
		return nil, errors.Errorf("no such resource '%s' in Modules", r.operation.Resource)
	}
	params, err := options.ListStructToParams(opts)
	if err != nil {
		return nil, err
	}
	ret, err := resourceManager.List(r.session, params)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(ret.Data))
	for i := range ret.Data {
		id, _ := ret.Data[i].GetString("id")
		name, _ := ret.Data[i].GetString("name")
		out[id] = name
	}
	return out, nil
}

func (r SAction) Apply(id string) (success bool, failReason string) {
	success = true
	resourceManager, ok := Modules[r.operation.Resource]
	if !ok {
		return false, fmt.Sprintf("no such resource '%s' in Modules", r.operation.Resource)
	}
	var requestFunc func(session *mcclient.ClientSession, id string, params *jsonutils.JSONDict) error

	action := utils.CamelSplit(r.operation.Operation, "-")
	requestFunc = func(session *mcclient.ClientSession, id string, params *jsonutils.JSONDict) error {
		if params == nil {
			params = jsonutils.NewDict()
		}
		_, err := resourceManager.PerformAction(session, id, action, params)
		return err
	}
	err := requestFunc(r.session, id, r.defautParams)
	if err != nil {
		clientErr, _ := err.(*httputils.JSONClientError)
		return false, clientErr.Details
	}
	if len(r.operation.StatusSuccess) == 0 {
		return true, ""
	}
	// wait for status
	timer := time.NewTimer(r.timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		timer.Stop()
	}()
	for {
		select {
		default:
			ret, e := resourceManager.GetSpecific(r.session, id, "status", nil)
			if e != nil {
				log.Errorf("fail to exec resouce(%s.%s).GetStatus: %s", r.operation.Resource, id, e.Error())
				<-ticker.C
				continue
			}
			status, _ := ret.GetString("status")
			if utils.IsInStringArray(status, r.operation.StatusSuccess) {
				return
			}
			for _, fail := range r.operation.Fail {
				if status != fail.Status {
					continue
				}
				params := jsonutils.NewDict()
				params.Add(jsonutils.NewString(id), "obj_id")
				params.Add(jsonutils.NewStringArray([]string{fail.LogEvent}), "action")
				params.Add(jsonutils.NewInt(1), "limit")
				events, err := modules.Logs.List(r.session, params)
				if err != nil {
					log.Errorf("Logs.List failed: %s", err.Error())
					<-ticker.C
					continue
				}
				if len(events.Data) == 0 {
					log.Errorf("These is no opslog about action '%s' for %s.%s: %s", fail.LogEvent, r.operation.Resource, id, err.Error())
					<-ticker.C
					continue
				}
				reason, _ := events.Data[0].GetString("notes")
				return false, reason
			}
			<-ticker.C
		case <-timer.C:
			log.Errorf("timeout(%s) to exec resource(%s.%s).%s", r.timeout.String(), r.operation.Resource, id, r.operation.Operation)
			return false, "timeout"
		}
	}
}
