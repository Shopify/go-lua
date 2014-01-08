%{

package lua

%}

%token BREAK GOTO NAME DO END WHILE REPEAT UNTIL IF THEN ELSE ELSEIF FOR IN FUNCTION LOCAL RETURN COLONCOLON NIL FALSE TRUE NUMBER STRING DOTDOTDOT upop DOTDOT LE GE EQ NE AND OR NOT

%left OR
%left AND
%left '<' '>' LE GE NE EQ
%right DOTDOT
%left '+' '-'
%left '*' '/' '%'
%left NOT '#' UMINUS
%right '^'

%%

chunk             : block
                  ;

block             : retstat
                  | statlist retstat
                  | /* nothing */
                  ;

statlist          : stat
                  | statlist stat
                  ;

stat              : ';'
                  | varlist '=' explist
                  | functioncall
                  | label
                  | BREAK
                  | GOTO NAME
                  | DO block END
                  | WHILE exp DO block END
                  | REPEAT block UNTIL exp
                  | ifstat
                  | FOR NAME '=' exp ',' exp DO block END
                  | FOR NAME '=' exp ',' exp ',' exp DO block END
                  | FOR namelist IN explist DO block END
                  | FUNCTION funcname funcbody
                  | LOCAL FUNCTION NAME funcbody
                  | LOCAL namelist
                  | LOCAL namelist '=' explist
                  ;

ifstat            : IF exp THEN block END
                  | IF exp THEN block elseif ELSE block END
                  ;

elseif            : ELSEIF exp THEN block
                  | elseif ELSEIF exp THEN block
                  ;

retstat           : RETURN
                  | RETURN ';'
                  | RETURN explist
                  | RETURN explist ';'
                  ;

label             : COLONCOLON NAME COLONCOLON
                  ;

funcname          : selectorlist
                  | selectorlist ':' NAME
                  ;

selectorlist      : NAME
                  | selectorlist '.' NAME
                  ;

varlist           : variable
                  | varlist ',' variable
                  ;

variable          : NAME
                  | prefixexp '[' exp ']'
                  | prefixexp '.' NAME
                  ;

namelist          : NAME
                  | namelist ',' NAME
                  ;

explist           : exp
                  | explist ',' exp
                  ;

exp               : NIL
                  | FALSE
                  | TRUE
                  | NUMBER
                  | STRING
                  | DOTDOTDOT
                  | functiondef
                  | prefixexp
                  | tableconstructor
                  | '-' exp %prec UMINUS
                  | NOT exp
                  | '^' exp
                  | exp binop exp
                  ;

prefixexp         : variable
                  | functioncall
                  | '(' exp ')'
                  ;

functioncall      : prefixexp args
                  | prefixexp ':' NAME args
                  ;

args              : '(' ')'
                  | '(' explist ')'
                  | tableconstructor
                  | STRING
                  ;

functiondef       : FUNCTION funcbody
                  ;

funcbody          : '(' ')' block END
                  | '(' parlist ')' block END
                  ;

parlist           : namelist
                  | namelist ',' DOTDOTDOT
                  | DOTDOTDOT
                  ;

tableconstructor  : '{' '}'
                  | '{' fieldlist '}'
                  ;

fieldlist         : field fieldrest
                  | field fieldrest fieldsep
                  ;

fieldrest         : fieldsep field
                  | fieldrest fieldsep field
                  | /* nothing */
                  ;

field             : '[' exp ']' '=' exp
                  | NAME '=' exp
                  | exp
                  ;

fieldsep          : ',' | ';' ;

binop             : '+' | '-' | '*' | '/' | '^' | '%' | DOTDOT | '<' | LE | '>' | GE | EQ | NE | AND | OR ;

%%
