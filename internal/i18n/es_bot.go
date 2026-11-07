package i18n

// Telegram bot and plan PRs.
func init() {
	for k, v := range map[string]string{
		"Approve":        "Aprobar",
		"Diff of run %d": "Diff de la ejecución %d",
		"No changes to show (the worktree may be gone).": "No hay cambios para mostrar (puede que el worktree ya no exista).",
		"No log yet.":   "Aún no hay log.",
		"No plans yet.": "Aún no hay planes.",
		"Nothing running and nothing waiting for you.": "No hay nada corriendo ni nada esperándote.",
		"PR open":           "PR abierto",
		"Plan %d not found": "No encontré el plan %d",
		"Plan %s paused: the running step finishes, the next one waits.": "Plan %s en pausa: el paso en curso termina y el siguiente espera.",
		"Plan %s started": "Plan %s iniciado",
		"Plan closed: its worktrees and branches were removed. Pull %s to get the merged PR.": "Plan cerrado: se borraron sus worktrees y ramas. Haz pull de %s para traer el PR mergeado.",
		"Project %s not found. Use its id or slug.":                                           "No encontré el proyecto %s. Usa su id o su slug.",
		"Reply to this message with what %s should change in “%s”.":                           "Responde a este mensaje con lo que %s debe cambiar en “%s”.",
		"Run %d not found":                   "No encontré la ejecución %d",
		"Send /help to see what I can do.":   "Envía /help para ver lo que puedo hacer.",
		"Sent to %s: “%s” is running again.": "Enviado a %s: “%s” está corriendo otra vez.",
		"Task #%d added to %s's backlog: %s": "Tarea #%d agregada al backlog de %s: %s",
		"The buttons under each alert approve, request changes, or show the diff and the log. Merging stays in the TUI.": "Los botones de cada aviso aprueban, piden cambios o muestran el diff y el log. El merge se hace en la TUI.",
		"The message was empty: nothing was sent.":               "El mensaje estaba vacío: no se envió nada.",
		"Unknown command. Send /help.":                           "No conozco ese comando. Envía /help.",
		"Usage: /diff <run id>":                                  "Uso: /diff <id de ejecución>",
		"Usage: /log <run id>":                                   "Uso: /log <id de ejecución>",
		"Usage: /pause <plan id>":                                "Uso: /pause <id del plan>",
		"Usage: /plan <id>":                                      "Uso: /plan <id>",
		"Usage: /run <plan id>":                                  "Uso: /run <id del plan>",
		"Usage: /task <project> <title>":                         "Uso: /task <proyecto> <título>",
		"add a task to a project's backlog":                      "agrega una tarea al backlog de un proyecto",
		"merge it on the remote, then press g again to clean up": "mergéalo en el remoto y luego pulsa g otra vez para limpiar",
		"pause a plan":                                           "pausa un plan",
		"plans and their progress":                               "planes y su progreso",
		"start or resume a plan":                                 "inicia o retoma un plan",
		"the steps of a plan":                                    "los pasos de un plan",
		"what is running, waiting for review or blocked":         "qué está corriendo, esperando revisión o bloqueado",
	} {
		es[k] = v
	}
}
