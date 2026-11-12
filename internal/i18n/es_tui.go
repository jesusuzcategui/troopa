package i18n

// Run screen tabs and scrolling panes.
func init() {
	for k, v := range map[string]string{
		"Log":                                   "Log",
		"tab/1 2 3 view":                        "tab/1 2 3 vista",
		"↓ %d more lines (j/k, PgDn)":           "↓ %d líneas más (j/k, AvPág)",
		"j/k scroll  PgUp/PgDn page  [ ] round": "j/k desplazar  RePág/AvPág página  [ ] ronda",
		"j/k scroll  PgUp/PgDn page  r fold  [ ] round":                       "j/k desplazar  RePág/AvPág página  r plegar  [ ] ronda",
		"j/k line  n/p file  enter comment  e edit  D delete  C comments":     "j/k línea  n/p archivo  enter comentar  e editar  D borrar  C comentarios",
		"g merge  O PR  x request changes  c clean  d discard":                "g merge  O PR  x pedir cambios  c limpiar  d descartar",
		"y approve  x request changes  d discard":                             "y aprobar  x pedir cambios  d descartar",
		"o editor  esc board":                                                 "o editor  esc tablero",
		"This run is %s: nothing left to approve or merge":                    "Esta ejecución está en %s: no queda nada que aprobar ni mezclar",
		"Last answer from the agent":                                          "Última respuesta del agente",
		"PgUp/PgDn scroll":                                                    "RePág/AvPág desplazar",
		"ctrl+s send to the agent · PgUp/PgDn scroll the answer · esc cancel": "ctrl+s enviar al agente · RePág/AvPág mover la respuesta · esc cancelar",
		"… v show the whole goal":                                             "… v ver el objetivo completo",
		"enter open step · E edit step · a add step · K/J move · s start · p pause · t auto · A auto-approve · e edit plan · v goal · g merge · O PR · U pull · esc plans": "enter abrir paso · E editar paso · a agregar paso · K/J mover · s iniciar · p pausar · t auto · A auto-aprobar · e editar plan · v objetivo · g merge · O PR · U pull · esc planes",
		"↓ %d more lines (PgDn)":                                              "↓ %d líneas más (AvPág)",
		"No log: the worktree of this run was removed.":                       "Sin log: el worktree de esta ejecución ya se borró.",
		"m status · P priority · A agent · tab edit fields · ctrl+d/u scroll": "m estado · P prioridad · A agente · tab editar campos · ctrl+d/u desplazar",
		"j/k field · ←/→ change · J/K attachment · a attach · d remove · o open · e edit all · PgUp/PgDn scroll · esc back":                                                             "j/k campo · ←/→ cambiar · J/K adjunto · a adjuntar · d quitar · o abrir · e editar todo · RePág/AvPág desplazar · esc volver",
		"enter open · n new · e edit · m status · P priority · A agent · [ ] move · tab panel · space/z fold · v board view · w plans · / search · B brain · o editor · ? help":         "enter abrir · n nueva · e editar · m estado · P prioridad · A agente · [ ] mover · tab panel · espacio/z plegar · v vista tablero · w planes · / buscar · B cerebro · o editor · ? ayuda",
		"enter open · n new · e edit · m status · P priority · A agent · [ ] move · tab panel · z fold · Z fold empty · v list view · w plans · / search · B brain · o editor · ? help": "enter abrir · n nueva · e editar · m estado · P prioridad · A agente · [ ] mover · tab panel · z plegar · Z plegar vacíos · v vista lista · w planes · / buscar · B cerebro · o editor · ? ayuda",
	} {
		es[k] = v
	}
}
