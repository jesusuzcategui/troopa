package i18n

// Chat notifications (troopa notify).
func init() {
	for k, v := range map[string]string{
		"%d of %d steps done":   "%d de %d pasos listos",
		"Agent started":         "Agente en marcha",
		"Approved":              "Aprobado",
		"Blocked: it needs you": "Bloqueado: te necesita",
		"Checks":                "Checks",
		"Discarded":             "Descartado",
		"In Troopa: y approves, x sends it back with your comments.": "En Troopa: y aprueba, x lo devuelve con tus comentarios.",
		"Merged":        "Mergeado",
		"Next step: %s": "Siguiente paso: %s",
		"Open the plan (w) and press g to merge it.":                         "Abre el plan (w) y pulsa g para hacer merge.",
		"Open the task in Troopa and press x to tell the agent what to fix.": "Abre la tarea en Troopa y pulsa x para decirle al agente qué arreglar.",
		"Plan done: ready to merge":                                          "Plan terminado: listo para merge",
		"Plan started":                                                       "Plan iniciado",
		"Ready for review":                                                   "Listo para revisar",
		"What the agent says":                                                "Lo que dice el agente",
		"step %d of %d":                                                      "paso %d de %d",
	} {
		es[k] = v
	}
}
