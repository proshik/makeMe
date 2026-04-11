-- +goose Up
-- One explanation per category; covers all questions added in seeds 11, 14, 15.

UPDATE grammar_questions SET explanation = 'Глагол «to be» согласуется с подлежащим: I am, he/she/it is, we/you/they are.' WHERE category = 'am/is/are';
UPDATE grammar_questions SET explanation = 'Present Simple используется для привычных действий и фактов. С he/she/it добавляем -s/-es: she goes, he watches.' WHERE category = 'Present Simple';
UPDATE grammar_questions SET explanation = 'Present Simple с have: I/you/we/they have, but he/she/it has.' WHERE category = 'Present Simple / have';
UPDATE grammar_questions SET explanation = 'Present Continuous (am/is/are + -ing) — действие происходит прямо сейчас или запланировано в ближайшем будущем.' WHERE category = 'Present Continuous';
UPDATE grammar_questions SET explanation = 'Present Simple — регулярные действия и факты; Present Continuous — действие происходит сейчас. Глаголы состояния (know, like, want) не употребляются в Continuous.' WHERE category = 'Present Simple vs Present Continuous';
UPDATE grammar_questions SET explanation = 'Past Simple — действие, завершённое в прошлом. Правильные глаголы: +ed. Неправильные нужно запоминать: go → went, see → saw.' WHERE category = 'Past Simple';
UPDATE grammar_questions SET explanation = 'Past Simple неправильных глаголов нужно запоминать отдельно: buy → bought, bring → brought, write → wrote.' WHERE category = 'Past Simple (irregular)';
UPDATE grammar_questions SET explanation = 'Past Continuous (was/were + -ing) — действие было в процессе в определённый момент прошлого. Часто сочетается с Past Simple: I was reading when he called.' WHERE category = 'Past Continuous';
UPDATE grammar_questions SET explanation = 'Present Perfect (have/has + V3) связывает прошлое с настоящим: результат важен сейчас. Ключевые слова: already, yet, just, ever, never.' WHERE category = 'Present Perfect';
UPDATE grammar_questions SET explanation = 'Present Perfect Continuous (have/has been + -ing) — действие началось в прошлом и продолжается или только что закончилось. Подчёркивает процесс: I have been waiting for an hour.' WHERE category = 'Present Perfect Continuous';
UPDATE grammar_questions SET explanation = 'Past Perfect (had + V3) — действие, завершённое до другого прошедшего момента: She had left before he arrived.' WHERE category = 'Past Perfect';
UPDATE grammar_questions SET explanation = 'will используется для спонтанных решений, предсказаний и обещаний. going to — для планов и очевидных намерений.' WHERE category = 'will (future)';
UPDATE grammar_questions SET explanation = 'going to + инфинитив выражает намерение или запланированное действие: I am going to visit Paris next month.' WHERE category = 'Going to';
UPDATE grammar_questions SET explanation = 'Future Plans: will — спонтанное решение; going to — заранее запланированное; Present Continuous — конкретно договорённое.' WHERE category = 'Future Plans';
UPDATE grammar_questions SET explanation = 'Future Perfect (will have + V3) — действие будет завершено к определённому моменту в будущем: By Friday I will have finished the report.' WHERE category = 'Future Perfect';
UPDATE grammar_questions SET explanation = 'Future Perfect Passive: will have been + V3. Действие будет завершено к будущему моменту в страдательном залоге.' WHERE category = 'Future Perfect Passive';
UPDATE grammar_questions SET explanation = 'Модальные глаголы (can, must, should, could, may, might, shall, will, would) не изменяются и требуют инфинитива без «to».' WHERE category = 'Modal Verbs';
UPDATE grammar_questions SET explanation = 'must и have to выражают обязанность. must — личное убеждение говорящего; have to — внешняя необходимость. needn''t — отсутствие необходимости.' WHERE category = 'must / have to / needn''t';
UPDATE grammar_questions SET explanation = 'must выражает обязанность или сильную уверенность; have to — необходимость из-за внешних обстоятельств; don''t have to — нет необходимости (не запрещено).' WHERE category = 'must/have to/don''t have to';
UPDATE grammar_questions SET explanation = 'should и ought to выражают совет или рекомендацию: You should see a doctor. Они взаимозаменяемы, но should употребляется чаще.' WHERE category = 'should / ought to';
UPDATE grammar_questions SET explanation = 'should have / could have / might have + V3 — оценка прошлых действий: You should have called (но не позвонил). Could have — была возможность.' WHERE category = 'should have / could have / might have';
UPDATE grammar_questions SET explanation = 'Пассивный залог (Passive Voice) — когда действие важнее его исполнителя. Образуется: be + V3. Подлежащее — получатель действия.' WHERE category = 'Passive Voice';
UPDATE grammar_questions SET explanation = 'Сложные пассивные конструкции: have something done (причинный залог) — заказывать выполнение чего-либо: I had my hair cut.' WHERE category = 'Complex Passive';
UPDATE grammar_questions SET explanation = 'Causative: have/get + объект + V3 — заставить кого-то сделать что-то: She had her car repaired.' WHERE category = 'Causative';
UPDATE grammar_questions SET explanation = 'Reported Speech: прямая речь превращается в косвенную, времена сдвигаются назад (backshift): "I am" → He said he was.' WHERE category = 'Reported Speech';
UPDATE grammar_questions SET explanation = 'First Conditional: If + Present Simple, will + инфинитив. Реальное условие в будущем: If it rains, I will stay home.' WHERE category = 'First conditional';
UPDATE grammar_questions SET explanation = 'First Conditional: If + Present Simple, will + инфинитив. Реальное условие в будущем.' WHERE category = 'Conditionals' AND level IN ('A2', 'B1');
UPDATE grammar_questions SET explanation = 'Second Conditional: If + Past Simple, would + инфинитив. Нереальное или маловероятное условие в настоящем/будущем.' WHERE category = 'Second Conditional';
UPDATE grammar_questions SET explanation = 'Second Conditional: If + Past Simple, would + инфинитив. Нереальное условие.' WHERE category = 'Second conditional';
UPDATE grammar_questions SET explanation = 'Third Conditional: If + Past Perfect, would have + V3. Нереальное условие о прошлом: If I had studied, I would have passed.' WHERE category = 'Third Conditional';
UPDATE grammar_questions SET explanation = 'Third Conditional: If + Past Perfect, would have + V3. Нереальное прошедшее условие.' WHERE category = 'Third conditional';
UPDATE grammar_questions SET explanation = 'Mixed Conditional: условие в прошлом + результат в настоящем (или наоборот).' WHERE category = 'Mixed Conditional';
UPDATE grammar_questions SET explanation = 'Complex Conditionals включают инверсию (Had I known...), смешанные времена и конструкции с unless, provided that, as long as.' WHERE category = 'Complex Conditionals';
UPDATE grammar_questions SET explanation = 'Conditional Inversion: Had I known... = If I had known... Используется в формальном стиле.' WHERE category = 'Conditional Inversion';
UPDATE grammar_questions SET explanation = 'Implied Conditionals выражают условие без союза if: otherwise, or, but for, without.' WHERE category = 'Implied Conditionals';
UPDATE grammar_questions SET explanation = 'Артикль a/an — неопределённый: a (перед согласным звуком), an (перед гласным). Используется при первом упоминании.' WHERE category = 'Articles: a/an';
UPDATE grammar_questions SET explanation = 'Определённый артикль the используется, когда говорящий и слушатель знают, о чём идёт речь, и с уникальными предметами.' WHERE category = 'Articles: the';
UPDATE grammar_questions SET explanation = 'Артикли: a/an — первое упоминание, the — известный предмет, нулевой артикль — с именами собственными, языками, едой (в общем смысле).' WHERE category = 'Articles';
UPDATE grammar_questions SET explanation = 'Существительные во множественном числе: +s/-es, неправильные формы (child → children, man → men). Неисчисляемые существительные не имеют мн. числа.' WHERE category = 'Plurals';
UPDATE grammar_questions SET explanation = 'Глагол «there is/are» вводит существование: There is a cat. There are two cats. Согласование по числу ближайшего существительного.' WHERE category = 'There is/are';
UPDATE grammar_questions SET explanation = 'there is / there are: There is (для ед. числа и неисчисляемых), There are (для мн. числа).' WHERE category = 'there is / there are';
UPDATE grammar_questions SET explanation = 'have (есть, иметь) и have got — синонимы в значении обладания. В американском английском have got менее употребителен.' WHERE category = 'Have/Have got';
UPDATE grammar_questions SET explanation = 'Местоимения: subject (I, he), object (me, him), possessive adjective (my, his), possessive pronoun (mine, his).' WHERE category = 'Pronouns';
UPDATE grammar_questions SET explanation = 'some — в утвердительных предложениях; any — в вопросах и отрицаниях. Исключение: some в вопросах-предложениях (Would you like some?).' WHERE category = 'Some/Any';
UPDATE grammar_questions SET explanation = 'Сравнительная степень: short → shorter than; long → more long... → more beautiful than.' WHERE category = 'Comparatives';
UPDATE grammar_questions SET explanation = 'Превосходная степень: short → the shortest; long → the most interesting. Не забывай артикль the.' WHERE category = 'Superlatives';
UPDATE grammar_questions SET explanation = 'Прилагательные стоят перед существительным; наречия изменяют глагол, прилагательное или другое наречие. Часто наречие = прилагательное + -ly.' WHERE category = 'Adjectives and adverbs';
UPDATE grammar_questions SET explanation = 'too + прилагательное/наречие (избыток): too cold to swim. Enough + существительное или прилагательное + enough: warm enough to swim.' WHERE category = 'Too/Enough';
UPDATE grammar_questions SET explanation = 'too/enough: too — слишком (негативно), enough — достаточно. Порядок: too + adj, adj + enough.' WHERE category = 'too / enough';
UPDATE grammar_questions SET explanation = 'so + прилагательное/наречие (so cold); such + (a) + прилагательное + существительное (such a cold day).' WHERE category = 'So/Such';
UPDATE grammar_questions SET explanation = 'Предлоги места: in (внутри), on (на поверхности), at (точка/место). Не переводятся дословно — запоминай устойчивые сочетания.' WHERE category = 'Prepositions of place';
UPDATE grammar_questions SET explanation = 'Предлоги времени: at (час: at 5), on (день: on Monday), in (период: in May, in 2020).' WHERE category = 'Prepositions of time';
UPDATE grammar_questions SET explanation = 'Предлоги управления глаголами и прилагательными нужно запоминать: interested in, good at, depend on, responsible for.' WHERE category = 'Prepositions';
UPDATE grammar_questions SET explanation = 'Question words: what (что/какой), who (кто), where (где/куда), when (когда), why (почему), how (как).' WHERE category = 'Question words';
UPDATE grammar_questions SET explanation = 'Question tags добавляются в конце: утверждение → отрицательный тег (She is, isn''t she?); отрицание → положительный тег (She isn''t, is she?).' WHERE category = 'Question Tags';
UPDATE grammar_questions SET explanation = 'Отрицания: don''t/doesn''t (Present Simple), didn''t (Past Simple), isn''t/aren''t, can''t и т.д. Двойное отрицание в английском недопустимо.' WHERE category = 'Negatives';
UPDATE grammar_questions SET explanation = 'used to + инфинитив — привычное прошлое действие или состояние, которого больше нет: I used to live here (но сейчас не живу).' WHERE category = 'used to';
UPDATE grammar_questions SET explanation = 'used to — прошлая привычка; be used to + -ing — привычность чего-то сейчас; get used to + -ing — привыкать.' WHERE category = 'used to / be used to / get used to';
UPDATE grammar_questions SET explanation = 'Герундий (глагол + -ing как существительное) после глаголов enjoy, avoid, keep, finish, suggest. Инфинитив (to + V) после want, hope, decide, plan.' WHERE category = 'Gerunds';
UPDATE grammar_questions SET explanation = 'Некоторые глаголы требуют герундия (enjoy doing), другие — инфинитива (want to do), третьи — оба с разным значением (stop doing vs stop to do).' WHERE category = 'Gerunds and Infinitives';
UPDATE grammar_questions SET explanation = 'Relative clauses определяют существительное. Defining (без запятых): who/that — для людей; which/that — для вещей.' WHERE category = 'Relative Clauses';
UPDATE grammar_questions SET explanation = 'Non-defining relative clauses дают дополнительную информацию и выделяются запятыми. that здесь не используется.' WHERE category = 'Non-defining relative clauses';
UPDATE grammar_questions SET explanation = 'Participle Clauses заменяют придаточные: Having finished his work, he left. (= After he had finished...)' WHERE category = 'Participle Clauses';
UPDATE grammar_questions SET explanation = 'would like + инфинитив — вежливая просьба или желание: I would like to order coffee.' WHERE category = 'would like';
UPDATE grammar_questions SET explanation = 'Stative Verbs (глаголы состояния: know, believe, want, like, see, smell) не используются в Continuous формах.' WHERE category = 'Stative Verbs';
UPDATE grammar_questions SET explanation = 'Подлежащее и сказуемое должны согласовываться в числе. Особые случаи: everyone is, the team are/is (брит./амер.).' WHERE category = 'Subject-Verb Agreement';
UPDATE grammar_questions SET explanation = 'Inversion (обратный порядок слов) используется после отрицательных наречий: Never have I seen..., Rarely does he...' WHERE category = 'Inversion';
UPDATE grammar_questions SET explanation = 'Temporal Inversion: Not until..., No sooner had... than..., Hardly had... when...' WHERE category = 'Temporal Inversion';
UPDATE grammar_questions SET explanation = 'Subjunctive (сослагательное наклонение): I suggest that he be present. Форма совпадает с базовым инфинитивом для всех лиц.' WHERE category = 'Subjunctive';
UPDATE grammar_questions SET explanation = 'Mandative Subjunctive после глаголов требования/рекомендации (suggest, insist, demand, recommend): I recommend that he take...' WHERE category = 'Mandative Subjunctive';
UPDATE grammar_questions SET explanation = 'Wish + Past Simple — желание изменить настоящее: I wish I knew the answer. Wish + Past Perfect — сожаление о прошлом.' WHERE category = 'Wish';
UPDATE grammar_questions SET explanation = 'Wish/If only: + Past Simple (хочу изменить настоящее), + Past Perfect (сожалею о прошлом), + would (раздражение).' WHERE category = 'Wish/If only';
UPDATE grammar_questions SET explanation = 'As if / As though: He acts as if he were the boss. После as if глагол в прошедшем для нереального действия.' WHERE category = 'As if';
UPDATE grammar_questions SET explanation = 'Modal Perfect: must have + V3 (точная уверенность в прошлом), might/could have + V3 (возможность), should have + V3 (упрёк).' WHERE category = 'Modal Perfect';
UPDATE grammar_questions SET explanation = 'Advanced Modals: ought to, had better, would rather, be supposed to — оттенки обязательства, предпочтения, ожидания.' WHERE category = 'Advanced Modals';
UPDATE grammar_questions SET explanation = 'Ellipsis — пропуск уже упомянутых элементов для краткости: I can play and so can she. (= so can she play)' WHERE category = 'Ellipsis';
UPDATE grammar_questions SET explanation = 'Cleft Sentences выделяют часть высказывания: It was John who broke it. What I need is a holiday.' WHERE category = 'Cleft Sentences';
UPDATE grammar_questions SET explanation = 'Emphasis достигается через do/does/did: I do understand you. А также через Cleft Sentences и инверсию.' WHERE category = 'Emphasis';
UPDATE grammar_questions SET explanation = 'Nominal Clauses (существительные придаточные) выполняют функцию подлежащего или дополнения: What he said surprised everyone.' WHERE category = 'Nominal Clauses';
UPDATE grammar_questions SET explanation = 'Comparison Clauses: the more..., the better; as... as; not as... as.' WHERE category = 'Comparison Clauses';
UPDATE grammar_questions SET explanation = 'Purpose Clauses выражают цель: to, in order to, so that, so as to: I came early so that I could get a seat.' WHERE category = 'Purpose Clauses';
UPDATE grammar_questions SET explanation = 'Reason Clauses вводятся through because, since, as, due to, owing to: Since it was raining, we stayed home.' WHERE category = 'Reason Clauses';
UPDATE grammar_questions SET explanation = 'Concession: although/though/even though/whereas/while — уступительные союзы: Although it was cold, we went out.' WHERE category = 'Concession';
UPDATE grammar_questions SET explanation = 'Concessive Clauses: despite/in spite of + noun/-ing; although/even though + clause.' WHERE category = 'Concessive Clauses';
UPDATE grammar_questions SET explanation = 'Nominalization — превращение глаголов/прилагательных в существительные для академического/делового стиля: decide → decision, happy → happiness.' WHERE category = 'Nominalization';
UPDATE grammar_questions SET explanation = 'Hypothetical Past: I wish I had done... / If only she had told me... Выражает сожаление о прошлом.' WHERE category = 'Hypothetical Past';
UPDATE grammar_questions SET explanation = 'Hypothetical structures: It is time we left; I would rather you stayed; Suppose/Imagine you were...' WHERE category = 'Hypothetical structures';
UPDATE grammar_questions SET explanation = 'Fixed Expressions и идиомы нужно запоминать как единицы: by the way, on the other hand, as a matter of fact.' WHERE category = 'Fixed Expressions';
UPDATE grammar_questions SET explanation = 'Lexical Patterns — устойчивые глагольные/именные коллокации: make a decision, take a break, do homework, have a rest.' WHERE category = 'Lexical Patterns';
UPDATE grammar_questions SET explanation = 'Formal Register требует пассива, номинализаций, сложных союзов (therefore, nevertheless, furthermore) и избегания сокращений.' WHERE category = 'Formal Register';
UPDATE grammar_questions SET explanation = 'Absolute Constructions — обособлённые причастные обороты без союза: Weather permitting, we will go. His work done, he went home.' WHERE category = 'Absolute Constructions';

-- Catch-all for any remaining questions without an explanation (future-proofing)
UPDATE grammar_questions SET explanation = '' WHERE explanation = '' AND category NOT IN (
    'am/is/are','Present Simple','Present Simple / have','Present Continuous',
    'Present Simple vs Present Continuous','Past Simple','Past Simple (irregular)',
    'Past Continuous','Present Perfect','Present Perfect Continuous','Past Perfect',
    'will (future)','Going to','Future Plans','Future Perfect','Future Perfect Passive',
    'Modal Verbs','must / have to / needn''t','must/have to/don''t have to',
    'should / ought to','should have / could have / might have',
    'Passive Voice','Complex Passive','Causative','Reported Speech',
    'First conditional','Conditionals','Second Conditional','Second conditional',
    'Third Conditional','Third conditional','Mixed Conditional','Complex Conditionals',
    'Conditional Inversion','Implied Conditionals','Articles: a/an','Articles: the','Articles',
    'Plurals','There is/are','there is / there are','Have/Have got','Pronouns',
    'Some/Any','Comparatives','Superlatives','Adjectives and adverbs','Too/Enough',
    'too / enough','So/Such','Prepositions of place','Prepositions of time','Prepositions',
    'Question words','Question Tags','Negatives','used to','used to / be used to / get used to',
    'Gerunds','Gerunds and Infinitives','Relative Clauses','Non-defining relative clauses',
    'Participle Clauses','would like','Stative Verbs','Subject-Verb Agreement',
    'Inversion','Temporal Inversion','Subjunctive','Mandative Subjunctive',
    'Wish','Wish/If only','As if','Modal Perfect','Advanced Modals',
    'Ellipsis','Cleft Sentences','Emphasis','Nominal Clauses','Comparison Clauses',
    'Purpose Clauses','Reason Clauses','Concession','Concessive Clauses',
    'Nominalization','Hypothetical Past','Hypothetical structures',
    'Fixed Expressions','Lexical Patterns','Formal Register','Absolute Constructions'
);

-- +goose Down
UPDATE grammar_questions SET explanation = '';
