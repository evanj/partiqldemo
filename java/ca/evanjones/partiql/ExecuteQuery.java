package ca.evanjones.partiql;

import com.amazon.ion.IonSystem;
import com.amazon.ion.system.IonSystemBuilder;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.IOException;
import java.nio.CharBuffer;
import java.nio.file.Files;
import java.nio.file.Path;
import org.partiql.lang.CompilerPipeline;
import org.partiql.lang.eval.EvaluationSession;
import org.partiql.lang.eval.ExprValue;
import org.partiql.lang.util.ConfigurableExprValueFormatter;

/**
ExecuteQuery is a command line program to execute a PartiQL query.
*/
public class ExecuteQuery {
    public static final void main(String[] args) throws IOException {
        if (args.length != 1) {
            System.err.println("Usage: Pass environment path as the only arg; reads query from STDIN");
            System.exit(1);
        }
        final String envPath = args[0];

        // read the envionment and evaluate it
        final String envContents = Files.readString(Path.of(envPath));

        final IonSystem ion = IonSystemBuilder.standard().build();
        final CompilerPipeline compilerPipeline = CompilerPipeline.standard(ion);
        final EvaluationSession initSession = EvaluationSession.standard();
        ExprValue envEvaluated = compilerPipeline.compile(envContents).eval(initSession);

        // read the query from stdin and evaluate it
        final String query = readToString(System.in);

        final EvaluationSession envSession = EvaluationSession.builder()
            .globals(envEvaluated.getBindings())
            .build();
        ExprValue queryResult = compilerPipeline.compile(query).eval(envSession);

        // pretty print the result
        ConfigurableExprValueFormatter.getPretty().formatTo(queryResult, System.out);
        System.out.println();
    }

    private static final String readToString(InputStream in) throws IOException {
        // inspired by Guava's CharStreams.toString; 2048 chars = 4096 bytes
        final int BUF_SIZE = 2048;
        final CharBuffer buf = CharBuffer.allocate(BUF_SIZE);
        final StringBuilder out = new StringBuilder();

        try (InputStreamReader reader = new InputStreamReader(in, "UTF-8")) {
            while (reader.read(buf) != -1) {
                buf.flip();
                out.append(buf);
                buf.clear();
            }
        }
        return out.toString();
    }
}
