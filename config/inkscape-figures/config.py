def latex_template(name, title):
    return '\n'.join((r"\begin{figure}[ht]",
                      r"    \centering",
                      rf"    \incfig[1]{{{name}}}",
                      rf"    \caption{{{title}}}",
                      r"\end{figure}"))
