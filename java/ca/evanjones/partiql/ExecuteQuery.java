package ca.evanjones.partiql;

import com.amazon.ion.IonSystem;
import com.amazon.ion.system.IonSystemBuilder;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.CharBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import org.partiql.lang.CompilerPipeline;
import org.partiql.lang.eval.EvaluationSession;
import org.partiql.lang.eval.ExprValue;
import org.partiql.lang.SqlException;
import org.partiql.lang.util.ConfigurableExprValueFormatter;

/**
ExecuteQuery is a command line program to execute a PartiQL query.
*/
public class ExecuteQuery {
    private static final String SERVER_FLAG = "--server";

    public static final void main(String[] args) throws IOException {
        if (args.length != 1) {
            System.err.println("Usage: Pass environment path as the only arg; reads query from STDIN");
            System.exit(1);
        }
        final String envPath = args[0];
        if (envPath.equals(SERVER_FLAG)) {
            System.err.println("PartiQLQuery: running in server mode; reading requests from stdin ...");
            serverMode();
            return;
        }

        // read the envionment and query; then evaluate
        final String envContents = Files.readString(Path.of(envPath));
        final String query = readToString(System.in);

        final String result = execute(query, envContents);
        System.out.println(result);
    }

    private static final String execute(final String query, final String environment) {
        final IonSystem ion = IonSystemBuilder.standard().build();
        final CompilerPipeline compilerPipeline = CompilerPipeline.standard(ion);
        final EvaluationSession initSession = EvaluationSession.standard();

        ExprValue queryResult = null;
        try {
            final ExprValue envEvaluated = compilerPipeline.compile(environment).eval(initSession);

            final EvaluationSession envSession = EvaluationSession.builder()
                .globals(envEvaluated.getBindings())
                .build();
            queryResult = compilerPipeline.compile(query).eval(envSession);
        } catch (SqlException e) {
            return "Execution error: " + e.toString();
        }

        // pretty print the result
        final StringBuilder output = new StringBuilder();
        ConfigurableExprValueFormatter.getPretty().formatTo(queryResult, output);
        return output.toString();
    }

    private static final String readToString(InputStream in) throws IOException {
        // inspired by Guava's CharStreams.toString; 2048 chars = 4096 bytes
        final int BUF_SIZE = 2048;
        final CharBuffer buf = CharBuffer.allocate(BUF_SIZE);
        final StringBuilder out = new StringBuilder();

        try (InputStreamReader reader = new InputStreamReader(in, StandardCharsets.UTF_8)) {
            while (reader.read(buf) != -1) {
                buf.flip();
                out.append(buf);
                buf.clear();
            }
        }
        return out.toString();
    }

    private static final void serverMode() throws IOException {
        while (true) {
            final InputPair input = readInput(System.in);
            final String result = execute(input.query, input.environment);
            writeString(System.out, result);
        }
    }

    private static final int HEADER_INT_LEN = 4;

    private static final InputPair readInput(InputStream in) throws IOException {
        final ByteBuffer header = ByteBuffer.allocate(HEADER_INT_LEN * 2);
        header.order(ByteOrder.LITTLE_ENDIAN);
        int bytes = in.read(header.array());
        if (bytes != HEADER_INT_LEN*2) {
            throw new RuntimeException("short read returned bytes: " + bytes);
        }

        final int queryLen = header.getInt();
        final int environmentLen = header.getInt();
        System.err.println("PartiQLQuery reading query:" + queryLen + " env:" + environmentLen);

        final String query = readString(in, queryLen);
        final String environment = readString(in, environmentLen);
        return new InputPair(query, environment);
    }

    private static final String readString(InputStream in, int length) throws IOException {
        final byte[] buf = new byte[length];
        final int bytes = in.read(buf);
        if (bytes != length) {
            throw new RuntimeException("short read " + bytes);
        }
        return new String(buf, StandardCharsets.UTF_8);
    }

    private static final void writeString(OutputStream out, String s) throws IOException {
        final byte[] buf = s.getBytes(StandardCharsets.UTF_8);

        final ByteBuffer header = ByteBuffer.allocate(HEADER_INT_LEN);
        header.order(ByteOrder.LITTLE_ENDIAN);
        header.putInt(buf.length);

        out.write(header.array());
        out.write(buf);
    }

    private static final class InputPair {
        public final String query;
        public final String environment;

        public InputPair(final String query, final String environment) {
            if (environment == null) {
                throw new NullPointerException("environment must not be null");
            }
            if (environment == null) {
                throw new NullPointerException("environment must not be null");
            }
            this.query = query;
            this.environment = environment;
        }
    }
}
