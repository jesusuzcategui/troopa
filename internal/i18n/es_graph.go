package i18n

// Brain graph.
func init() {
	for k, v := range map[string]string{
		"%d links":                          "%d enlaces",
		"Brain graph":                       "Grafo del cerebro",
		"The second brain has no notes yet": "El segundo cerebro aún no tiene notas",
		"arrows move · enter local view · +/- zoom · 0 fit · v local view · r reload · esc back": "flechas mover · enter vista local · +/- zoom · 0 ajustar · v vista local · r recargar · esc volver",
		"guide": "guía",
		"hub":   "índice",
		"j/k tree  enter open  tab link  G graph  / search  o edit  i inbox  esc back": "j/k árbol  enter abrir  tab enlace  G grafo  / buscar  o editar  i inbox  esc volver",
		"local":       "local",
		"note":        "nota",
		"notes":       "notas",
		"project":     "proyecto",
		"task":        "tarea",
		"whole brain": "todo el cerebro",
		"←/→ pick · enter center / open · 1/2 rings · v whole brain · r reload · esc back": "←/→ elegir · enter centrar / abrir · 1/2 anillos · v todo el cerebro · r recargar · esc volver",
	} {
		es[k] = v
	}
}
