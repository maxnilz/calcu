%{
package calcu

func setErr(exprlex exprLexer, err error) int {
    exprlex.(*lexer).setErr(err)
    return 1
}

func setRoot(exprlex exprLexer, node Node) {
    exprlex.(*lexer).setRoot(node)
}

%}

%union {
    token int
    str string

    list *List
    node Node
}

%token<str> IDENT NUM UNIT LITERALSTR LITERALMV

%type<str> func_name
%type<list> func_arg_list
%type<node> a_expr func_call func_arg_expr assignment statement

%left '+' '-'
%left '*' '/'

%%

statement:/* empty */ {setRoot(exprlex, nil)}
         | func_call ';' {setRoot(exprlex, $1)}
         | assignment ';' {setRoot(exprlex, $1)}
         ;

a_expr: NUM UNIT
        {
          n, err := makeMeasureValue($1, $2)
          if err != nil {
              return setErr(exprlex, err)
          }
          $$ = n
        }
      | LITERALMV
        {
          n, err := makeLiteralMeasureValue($1)
          if err != nil {
              return setErr(exprlex, err)
          }
          $$ = n
        }
      | NUM
        {
          n, err := makeUnitlessMeasureValue($1)
          if err != nil {
              return setErr(exprlex, err)
          }
          $$ = n
        }
      | IDENT
        {
          $$ = makeVariable($1)
        }
      | a_expr '+' a_expr
        {
          $$ = makeBinaryExpr($1, $3, "+")
        }
      | a_expr '-' a_expr
        {
          $$ = makeBinaryExpr($1, $3, "-")
        }
      | a_expr '*' a_expr
        {
          $$ = makeBinaryExpr($1, $3, "*")
        }
      | a_expr '/' a_expr
        {
          $$ = makeBinaryExpr($1, $3, "/")
        }
      | '-' a_expr %prec '*'
        {
          $$ = makeUnaryExpr($2)
        }
      ;

func_call: func_name '(' ')'
           {
             n, err := makeFuncCall($1)
             if err != nil {
                 return setErr(exprlex, err)
             }
             $$ = n
           }
         | func_name '(' func_arg_list ')'
           {
             n, err := makeFuncCall($1, $3.elements...)
             if err != nil {
                 return setErr(exprlex, err)
             }
             $$ = n
           }
         ;

func_name: IDENT
          ;

func_arg_list: func_arg_expr
               {
                 l := makeList()
                 l.Append($1)
                 $$ = l
               }
	     | func_arg_list ',' func_arg_expr
               {
                 $$.Append($3)
               }
	     ;

func_arg_expr: IDENT
               {
                 $$ = makeVariable($1)
               }
             | LITERALMV
               {
                 n, err := makeLiteralMeasureValue($1)
                 if err != nil {
                     return setErr(exprlex, err)
                 }
                 $$ = n
               }
             | LITERALSTR
               {
                 $$ = makeLiteralString($1)
               }
             | a_expr
               {
                 $$ = $1
               }
             ;

assignment: IDENT '=' a_expr
            {
              n, err := makeAssignment($1, $3)
              if err != nil {
                  return setErr(exprlex, err)
              }
              $$ = n
            }
	  | IDENT '=' func_call
            {
              n, err := makeAssignment($1, $3)
              if err != nil {
                  return setErr(exprlex, err)
              }
              $$ = n
            }
           ;

%%
