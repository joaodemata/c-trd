package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
)

// Runner ejecuta los agentes 1-3 sobre las tesis de la cola: crea las condiciones que faltan, arma el trigger con
// todas las condiciones existentes, escribe el Pine y crea el trigger. Todo queda pendiente de revision del super
// usuario. El agente 4 (backtest) queda para despues
type Runner struct {
	Client      *CtrdClient
	LLM         LLM
	MaxAttempts int // intentos por agente
}

// thesisRun estado de una tesis mientras la procesan los agentes
type thesisRun struct {
	start        RunStart
	steps        []RunStep
	attempts     map[string]int
	idConditions []string
}

// Run toma hasta 5 tesis y las procesa en orden. Si Claude o c-trd fallan, la tesis actual y las que faltan vuelven
// a la cola (requeue) para la siguiente semana
func (r *Runner) Run(ctx context.Context) *cmm.ErrorHandler {
	runs, err := r.Client.StartRun(ctx)
	if !err.IsEmtpy() {
		return err
	}
	if len(runs) == 0 {
		log.Println("[AGENTS] No hay tesis pendientes")
		return cmm.NewEmptyErrorHandler()
	}

	for i, run := range runs {
		log.Printf("[AGENTS] Tesis %d/%d: %s %s (corrida %s)", i+1, len(runs), run.Thesis.TagAction, run.Thesis.TagAsset, run.IDRun)
		// fatal: Claude no responde, se acabo el uso del plan o c-trd esta caido
		finish, fatal := r.processThesis(ctx, run)
		if !fatal.IsEmtpy() {
			r.requeueRemaining(ctx, runs[i:], fatal)
			return fatal
		}

		if errFinish := r.Client.FinishRun(ctx, finish); !errFinish.IsEmtpy() {
			log.Printf("[AGENTS] No se pudo cerrar la corrida %s: %s", run.IDRun, errFinish.Message)
			continue
		}
		log.Printf("[AGENTS] Corrida %s: %s", run.IDRun, finish.TagStatus)
	}
	return cmm.NewEmptyErrorHandler()
}

// requeueRemaining cierra como fallidas las corridas que no se completaron y devuelve sus tesis a la cola
func (r *Runner) requeueRemaining(ctx context.Context, runs []RunStart, cause *cmm.ErrorHandler) {
	for _, run := range runs {
		finish := FinishRunRequest{
			IdRun:      run.IDRun,
			TagStatus:  cmm.AgentRunStatusFailed.Tag,
			Steps:      []RunStep{},
			Conditions: []string{},
			Error:      truncate("Corrida detenida, la tesis vuelve a la cola: "+cause.Message, 4000),
			Requeue:    true,
		}
		if err := r.Client.FinishRun(ctx, finish); !err.IsEmtpy() {
			log.Printf("[AGENTS] No se pudo devolver la tesis de la corrida %s: %s", run.IDRun, err.Message)
		}
	}
}

func (r *Runner) maxAttempts() int {
	if r.MaxAttempts <= 0 {
		return 3
	}
	return r.MaxAttempts
}

// ---------- Contexto que reciben los agentes (lo trae el runner, los agentes no tienen herramientas) ----------

type agentContext struct {
	Conditions []models.ConditionsModelType
	Apis       []models.ApisModelType
	Callbacks  map[string][]providers.CallbackInfo // por response_format
	Catalogs   map[string][]models.CatalogsModelType
}

var agentCatalogTypes = []string{
	cmm.CatalogTypeCondition, cmm.CatalogTypeVisibility, cmm.CatalogTypeAction, cmm.CatalogTypeCandleStick, cmm.CatalogTypeCancelReason,
}

func (r *Runner) loadContext(ctx context.Context) (agentContext, *cmm.ErrorHandler) {
	data := agentContext{Callbacks: map[string][]providers.CallbackInfo{}, Catalogs: map[string][]models.CatalogsModelType{}}

	conditions, err := r.Client.ListConditions(ctx)
	if !err.IsEmtpy() {
		return data, err
	}
	data.Conditions = conditions

	apis, err := r.Client.ListApis(ctx)
	if !err.IsEmtpy() {
		return data, err
	}
	data.Apis = apis

	for _, api := range apis {
		if _, loaded := data.Callbacks[api.ResponseFormat]; loaded || api.ResponseFormat == "" {
			continue
		}
		callbacks, err := r.Client.ListCallbacks(ctx, api.ResponseFormat)
		if !err.IsEmtpy() {
			return data, err
		}
		data.Callbacks[api.ResponseFormat] = callbacks
	}

	for _, typeCatalog := range agentCatalogTypes {
		catalogs, err := r.Client.ListCatalogs(ctx, typeCatalog)
		if !err.IsEmtpy() {
			return data, err
		}
		data.Catalogs[typeCatalog] = catalogs
	}
	return data, cmm.NewEmptyErrorHandler()
}

// conditionSummary condicion como la ven los agentes
type conditionSummary struct {
	IdCondition        string         `json:"idCondition"`
	Tag                string         `json:"tag"`
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	NameProvider       string         `json:"nameProvider"`
	TagTypeCondition   string         `json:"tagTypeCondition"`
	IdApi              string         `json:"idApi"`
	CallbackFunction   string         `json:"callbackFunction"`
	ParametersCallback map[string]any `json:"parametersCallback"`
	IntervalSeconds    *int           `json:"intervalSeconds"`
	TagStatus          string         `json:"tagStatus"`
}

func summarizeConditions(conditions []models.ConditionsModelType) []conditionSummary {
	summaries := make([]conditionSummary, 0, len(conditions))
	for _, c := range conditions {
		summaries = append(summaries, conditionSummary{
			IdCondition: c.ID.Hex(), Tag: c.Tag, Name: c.Name, Description: c.Description, NameProvider: c.NameProvider,
			TagTypeCondition: c.TagTypeCondition, IdApi: c.IDApi.Hex(), CallbackFunction: c.CallbackFunction,
			ParametersCallback: c.ParametersCallback, IntervalSeconds: c.IntervalSeconds, TagStatus: c.TagStatus,
		})
	}
	return summaries
}

// usableConditions condiciones que un trigger puede usar: sin las que el super usuario rechazo
func usableConditions(conditions []models.ConditionsModelType) []models.ConditionsModelType {
	usable := make([]models.ConditionsModelType, 0, len(conditions))
	for _, condition := range conditions {
		if condition.TagStatus != cmm.GenericStatusNotApproved.Tag {
			usable = append(usable, condition)
		}
	}
	return usable
}

// catalogOption catalogo como lo ve el agente
type catalogOption struct {
	Id   string `json:"id"`
	Tag  string `json:"tag"`
	Name string `json:"name"`
}

func (a agentContext) catalogOptions(typeCatalog string) []catalogOption {
	options := []catalogOption{}
	for _, catalog := range a.Catalogs[typeCatalog] {
		options = append(options, catalogOption{Id: catalog.ID.Hex(), Tag: catalog.Tag, Name: catalog.Name})
	}
	return options
}

func (a agentContext) catalogTag(typeCatalog string, id string) string {
	for _, catalog := range a.Catalogs[typeCatalog] {
		if catalog.ID.Hex() == id {
			return catalog.Tag
		}
	}
	return ""
}

type thesisInput struct {
	IdAsset   string `json:"idAsset"`
	TagAsset  string `json:"tagAsset"`
	Asset     string `json:"asset"`
	IdAction  string `json:"idAction"`
	TagAction string `json:"tagAction"`
	Thesis    string `json:"thesis"`
}

type previousAttempt struct {
	Output   any      `json:"output"`
	Problems []string `json:"problems"`
}

// ---------- Ejecucion de una tesis ----------

func (r *Runner) processThesis(ctx context.Context, start RunStart) (FinishRunRequest, *cmm.ErrorHandler) {
	run := &thesisRun{start: start, steps: []RunStep{}, attempts: map[string]int{}, idConditions: []string{}}
	thesis := thesisInput{
		IdAsset: start.Thesis.IDAsset.Hex(), TagAsset: start.Thesis.TagAsset, Asset: start.Thesis.Asset,
		IdAction: start.Thesis.IDAction.Hex(), TagAction: start.Thesis.TagAction, Thesis: start.Thesis.Thesis,
	}

	data, err := r.loadContext(ctx)
	if !err.IsEmtpy() {
		return FinishRunRequest{}, err
	}

	// 1. Condition Designer. done indica que la corrida termino ahi (NEEDS_INPUT o FAILED)
	designer, finish, done, fatal := r.runConditionDesigner(ctx, run, thesis, data)
	if !fatal.IsEmtpy() {
		return FinishRunRequest{}, fatal
	}
	if done {
		return finish, cmm.NewEmptyErrorHandler()
	}

	// Las condiciones nuevas tambien estan disponibles para el Trigger Composer
	data.Conditions, err = r.Client.ListConditions(ctx)
	if !err.IsEmtpy() {
		return FinishRunRequest{}, err
	}

	// 2 y 3. Trigger Composer + Pine Author + creacion del trigger
	return r.runTrigger(ctx, run, thesis, data, designer)
}

// callAgent ejecuta un agente y decodifica su salida. Un problema de formato se devuelve como problema (se reintenta);
// solo un fallo de Claude es fatal
func callAgent[T any](ctx context.Context, r *Runner, prompt string, input any) (T, map[string]any, []string, *cmm.ErrorHandler) {
	var out T
	system, err := LoadPrompt(prompt)
	if !err.IsEmtpy() {
		return out, map[string]any{}, []string{}, err
	}

	encoded, errJSON := json.Marshal(input)
	if errJSON != nil {
		return out, map[string]any{}, []string{}, cmm.NewErrorHandler(errJSON, "No se pudo serializar la entrada del agente", cmm.LevelFatal, "AGRUNE001")
	}

	text, err := r.LLM.Complete(ctx, system, string(encoded))
	if !err.IsEmtpy() {
		return out, map[string]any{}, []string{}, err
	}

	out, raw, errDecode := DecodeAgentOutput[T](text)
	if !errDecode.IsEmtpy() {
		if len(raw) == 0 {
			raw = map[string]any{"raw": truncate(text, 4000)}
		}
		return out, raw, []string{errDecode.Message}, cmm.NewEmptyErrorHandler()
	}
	return out, raw, []string{}, cmm.NewEmptyErrorHandler()
}

func (r *Runner) recordStep(run *thesisRun, agent string, output map[string]any, problems []string) {
	run.attempts[agent]++
	if output == nil {
		output = map[string]any{}
	}
	limited := make([]string, 0, len(problems))
	for _, problem := range problems {
		limited = append(limited, truncate(problem, 4000))
	}
	run.steps = append(run.steps, RunStep{Agent: agent, Attempt: run.attempts[agent], Output: output, Problems: limited})
}

func (r *Runner) finish(run *thesisRun, status cmm.CatalogStatus, idTrigger string, message string) FinishRunRequest {
	return FinishRunRequest{
		IdRun:      run.start.IDRun,
		TagStatus:  status.Tag,
		Steps:      run.steps,
		Conditions: run.idConditions,
		IdTrigger:  idTrigger,
		Error:      truncate(message, 4000),
	}
}

type designerInput struct {
	Thesis             thesisInput                         `json:"thesis"`
	ExistingConditions []conditionSummary                  `json:"existingConditions"`
	Apis               []models.ApisModelType              `json:"apis"`
	Callbacks          map[string][]providers.CallbackInfo `json:"callbacksByResponseFormat"`
	TypeCondition      []catalogOption                     `json:"typeCondition"`
	TypeVisibility     []catalogOption                     `json:"typeVisibility"`
	AlreadyCreated     map[string]string                   `json:"alreadyCreated"`
	PreviousAttempt    *previousAttempt                    `json:"previousAttempt,omitempty"`
}

// runConditionDesigner crea las condiciones que faltan. Las que el backend acepta se guardan aunque otras fallen; en el
// siguiente intento el agente las recibe en alreadyCreated y solo corrige las que fallaron
func (r *Runner) runConditionDesigner(ctx context.Context, run *thesisRun, thesis thesisInput, data agentContext) (ConditionDesignerOutput, FinishRunRequest, bool, *cmm.ErrorHandler) {
	var out ConditionDesignerOutput
	created := map[string]string{} // tag -> id
	var previous *previousAttempt

	for attempt := 1; attempt <= r.maxAttempts(); attempt++ {
		input := designerInput{
			Thesis: thesis, ExistingConditions: summarizeConditions(data.Conditions), Apis: data.Apis, Callbacks: data.Callbacks,
			TypeCondition: data.catalogOptions(cmm.CatalogTypeCondition), TypeVisibility: data.catalogOptions(cmm.CatalogTypeVisibility),
			AlreadyCreated: created, PreviousAttempt: previous,
		}

		decoded, raw, problems, fatal := callAgent[ConditionDesignerOutput](ctx, r, PromptConditionDesigner, input)
		if !fatal.IsEmtpy() {
			return out, FinishRunRequest{}, true, fatal
		}
		out = decoded

		if len(problems) == 0 && out.Status == "NEEDS_INPUT" {
			r.recordStep(run, AgentConditionDesigner, raw, problems)
			return out, r.finish(run, cmm.AgentRunStatusNeedsInput, "", "Condition Designer: "+strings.Join(out.Questions, " | ")), true, cmm.NewEmptyErrorHandler()
		}
		if len(problems) == 0 {
			problems = ValidateDesignedConditions(out)
		}
		if len(problems) == 0 {
			problems = r.createConditions(ctx, run, out, created)
		}

		r.recordStep(run, AgentConditionDesigner, raw, problems)
		if len(problems) == 0 {
			return out, FinishRunRequest{}, false, cmm.NewEmptyErrorHandler()
		}
		previous = &previousAttempt{Output: raw, Problems: problems}
	}

	return out, r.finish(run, cmm.AgentRunStatusFailed, "", fmt.Sprintf("Condition Designer sin salida valida tras %d intentos", r.maxAttempts())), true, cmm.NewEmptyErrorHandler()
}

func (r *Runner) createConditions(ctx context.Context, run *thesisRun, out ConditionDesignerOutput, created map[string]string) []string {
	problems := []string{}
	for i, condition := range out.Conditions {
		if _, done := created[condition.Tag]; done {
			continue
		}
		id, err := r.Client.CreateCondition(ctx, condition.RequestBody())
		if !err.IsEmtpy() {
			problems = append(problems, fmt.Sprintf("conditions[%d] (%s): c-trd rechazo la condicion: %s", i, condition.Tag, err.Message))
			continue
		}
		created[condition.Tag] = id
		run.idConditions = append(run.idConditions, id)
	}
	return problems
}

type composerInput struct {
	Thesis               thesisInput        `json:"thesis"`
	Designer             any                `json:"designer"`
	Conditions           []conditionSummary `json:"conditions"`
	TypeAction           []catalogOption    `json:"typeAction"`
	TypeCandleStick      []catalogOption    `json:"typeCandleStick"`
	TypeVisibility       []catalogOption    `json:"typeVisibility"`
	PreviousAttempt      *previousAttempt   `json:"previousAttempt,omitempty"`
	ConditionsThisRunIds []string           `json:"conditionsCreatedThisRun"`
}

type pineTrigger struct {
	Tag                  string `json:"tag"`
	Name                 string `json:"name"`
	Action               string `json:"action"`
	CandlestickTimeframe string `json:"candlestickTimeframe"`
	MaxCandlestickQty    int    `json:"maxCandlestickQty"`
	TechnicalBrief       string `json:"technicalBrief"`
}

type pineInput struct {
	Thesis          thesisInput        `json:"thesis"`
	Trigger         pineTrigger        `json:"trigger"`
	Conditions      []conditionSummary `json:"conditions"`
	CancelReasons   []catalogOption    `json:"cancelReasons"`
	PreviousAttempt *previousAttempt   `json:"previousAttempt,omitempty"`
}

// runTrigger arma el trigger, escribe el Pine y lo crea. Si c-trd rechaza el trigger (ej: condiciones incompatibles o
// tag repetido) el Trigger Composer lo corrige; el Pine solo se reescribe si cambia la accion, la temporalidad o las velas
func (r *Runner) runTrigger(ctx context.Context, run *thesisRun, thesis thesisInput, data agentContext, designer ConditionDesignerOutput) (FinishRunRequest, *cmm.ErrorHandler) {
	var previous *previousAttempt
	var pine PineAuthorOutput
	var pineFor ComposedTrigger
	hasPine := false

	for attempt := 1; attempt <= r.maxAttempts(); attempt++ {
		input := composerInput{
			Thesis: thesis,
			Designer: map[string]any{
				"reusedConditions": designer.ReusedConditions, "technicalHints": designer.TechnicalHints,
				"createdConditions": designer.Conditions,
			},
			Conditions:           summarizeConditions(usableConditions(data.Conditions)),
			TypeAction:           data.catalogOptions(cmm.CatalogTypeAction),
			TypeCandleStick:      data.catalogOptions(cmm.CatalogTypeCandleStick),
			TypeVisibility:       data.catalogOptions(cmm.CatalogTypeVisibility),
			PreviousAttempt:      previous,
			ConditionsThisRunIds: run.idConditions,
		}

		out, raw, problems, fatal := callAgent[TriggerComposerOutput](ctx, r, PromptTriggerComposer, input)
		if !fatal.IsEmtpy() {
			return FinishRunRequest{}, fatal
		}
		if len(problems) == 0 && out.Status == "NEEDS_INPUT" {
			r.recordStep(run, AgentTriggerComposer, raw, problems)
			return r.finish(run, cmm.AgentRunStatusNeedsInput, "", "Trigger Composer: "+strings.Join(out.Questions, " | ")), cmm.NewEmptyErrorHandler()
		}

		timeframeTag := data.catalogTag(cmm.CatalogTypeCandleStick, out.Trigger.IdCandlestickTimeframe)
		if len(problems) == 0 {
			problems = ValidateComposedTrigger(out, usableConditions(data.Conditions), timeframeTag)
			if out.Trigger.IdAction != thesis.IdAction {
				problems = append(problems, fmt.Sprintf("trigger.idAction=%s no es la accion de la tesis (%s %s)", out.Trigger.IdAction, thesis.TagAction, thesis.IdAction))
			}
			if out.Trigger.IdAsset != thesis.IdAsset {
				problems = append(problems, fmt.Sprintf("trigger.idAsset=%s no es el asset de la tesis (%s)", out.Trigger.IdAsset, thesis.IdAsset))
			}
		}
		if len(problems) > 0 {
			r.recordStep(run, AgentTriggerComposer, raw, problems)
			previous = &previousAttempt{Output: raw, Problems: problems}
			continue
		}

		// 3. Pine Author, solo si no hay codigo o cambio lo que el codigo implementa
		if !hasPine || pineFor.IdAction != out.Trigger.IdAction || pineFor.IdCandlestickTimeframe != out.Trigger.IdCandlestickTimeframe || pineFor.MaxCandlestickQty != out.Trigger.MaxCandlestickQty {
			trigger := pineTrigger{
				Tag: out.Trigger.Tag, Name: out.Trigger.Name, Action: thesis.TagAction, CandlestickTimeframe: timeframeTag,
				MaxCandlestickQty: out.Trigger.MaxCandlestickQty, TechnicalBrief: out.TechnicalBrief,
			}
			written, finish, done, fatal := r.runPineAuthor(ctx, run, thesis, data, trigger, out.Trigger.Conditions)
			if !fatal.IsEmtpy() {
				return FinishRunRequest{}, fatal
			}
			if done {
				r.recordStep(run, AgentTriggerComposer, raw, []string{})
				finish.Steps = run.steps
				return finish, cmm.NewEmptyErrorHandler()
			}
			pine, pineFor, hasPine = written, out.Trigger, true
		}

		idTrigger, err := r.Client.CreateTrigger(ctx, out.Trigger.RequestBody(pine.PineIndicator))
		if !err.IsEmtpy() {
			problems = []string{"c-trd rechazo el trigger: " + err.Message}
			r.recordStep(run, AgentTriggerComposer, raw, problems)
			previous = &previousAttempt{Output: raw, Problems: problems}
			continue
		}

		r.recordStep(run, AgentTriggerComposer, raw, []string{})
		return r.finish(run, cmm.AgentRunStatusSuccess, idTrigger, ""), cmm.NewEmptyErrorHandler()
	}

	return r.finish(run, cmm.AgentRunStatusFailed, "", fmt.Sprintf("Trigger Composer sin trigger valido tras %d intentos", r.maxAttempts())), cmm.NewEmptyErrorHandler()
}

// runPineAuthor done indica que la corrida termino (FAILED) porque el agente no dio un codigo valido
func (r *Runner) runPineAuthor(ctx context.Context, run *thesisRun, thesis thesisInput, data agentContext, trigger pineTrigger, idConditions []string) (PineAuthorOutput, FinishRunRequest, bool, *cmm.ErrorHandler) {
	var out PineAuthorOutput
	var previous *previousAttempt

	chosen := []models.ConditionsModelType{}
	for _, condition := range data.Conditions {
		for _, id := range idConditions {
			if condition.ID.Hex() == id {
				chosen = append(chosen, condition)
			}
		}
	}

	for attempt := 1; attempt <= r.maxAttempts(); attempt++ {
		input := pineInput{
			Thesis: thesis, Trigger: trigger, Conditions: summarizeConditions(chosen),
			CancelReasons: data.catalogOptions(cmm.CatalogTypeCancelReason), PreviousAttempt: previous,
		}

		decoded, raw, problems, fatal := callAgent[PineAuthorOutput](ctx, r, PromptPineAuthor, input)
		if !fatal.IsEmtpy() {
			return out, FinishRunRequest{}, true, fatal
		}
		out = decoded
		if len(problems) == 0 {
			problems = ValidatePineOutput(out)
		}

		r.recordStep(run, AgentPineAuthor, raw, problems)
		if len(problems) == 0 {
			return out, FinishRunRequest{}, false, cmm.NewEmptyErrorHandler()
		}
		previous = &previousAttempt{Output: raw, Problems: problems}
	}

	return out, r.finish(run, cmm.AgentRunStatusFailed, "", fmt.Sprintf("Pine Author sin codigo valido tras %d intentos", r.maxAttempts())), true, cmm.NewEmptyErrorHandler()
}
