/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package storm.dataprocessing;

import com.rabbitmq.client.AMQP;
import com.rabbitmq.client.Channel;
import com.rabbitmq.client.Connection;
import com.rabbitmq.client.ConnectionFactory;
import com.rabbitmq.client.GetResponse;

import backtype.storm.Config;
import backtype.storm.LocalCluster;
import backtype.storm.StormSubmitter;
import backtype.storm.spout.Scheme;
import backtype.storm.spout.SpoutOutputCollector;
import backtype.storm.task.OutputCollector;
import backtype.storm.task.TopologyContext;
import backtype.storm.topology.IRichSpout;
import backtype.storm.topology.OutputFieldsDeclarer;
import backtype.storm.topology.TopologyBuilder;
import backtype.storm.topology.base.BaseRichBolt;
import backtype.storm.topology.base.BaseRichSpout;
import backtype.storm.tuple.Fields;
import backtype.storm.tuple.Tuple;
import backtype.storm.tuple.Values;
import backtype.storm.utils.Utils;
import io.latent.storm.rabbitmq.RabbitMQSpout;
import io.latent.storm.rabbitmq.config.ConnectionConfig;
import io.latent.storm.rabbitmq.config.ConsumerConfig;
import io.latent.storm.rabbitmq.config.ConsumerConfigBuilder;
import com.google.api.services.bigquery.Bigquery;
import com.google.api.services.bigquery.BigqueryScopes;
import com.google.api.services.bigquery.model.TableDataInsertAllRequest;
import com.google.api.services.bigquery.model.TableDataInsertAllResponse;
import com.google.api.client.googleapis.auth.oauth2.GoogleCredential;
import com.google.api.client.http.HttpTransport;
import com.google.api.client.http.javanet.NetHttpTransport;
import com.google.api.client.json.JsonFactory;
import com.google.api.client.json.jackson2.JacksonFactory;
import com.google.gson.Gson;
import com.google.gson.JsonSyntaxException;
import com.google.gson.stream.JsonReader;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Collections;
import java.util.Collection;
import java.io.StringReader;
import java.io.IOException;

/**
 * This is a basic example of a Storm topology.
 */
public class DataProcessingTopology {

  private static final String RABBIT_USERNAME = "rabbit";
  private static final String RABBIT_PASSWORD = "rabbit-password";
  private static final String RABBIT_HOST = "rabbitmq-service";
  private static final int RABBIT_PORT = 5672;
  // Replace with your project id
  private static final String PROJECT_ID = "dm-k8s-testing";
  private static final String DATASET_ID = "messages_dataset";
  private static final String TABLE_ID = "messages_dataset_table";
  private static Bigquery bigquery;

  /**
   * This class creates our Service to connect to Bigquery including auth.
   */
  public final static class BigqueryServiceFactory {

    /**
     * Private constructor to disable creation of this utility Factory class.
     */
    private BigqueryServiceFactory() {

    }

    /**
     * Singleton service used through the app.
     */
    private static Bigquery service = null;

    /**
     * Mutex created to create the singleton in thread-safe fashion.
     */
    private static Object serviceLock = new Object();

    /**
     * Threadsafe Factory that provides an authorized Bigquery service.
     * @return The Bigquery service
     * @throws IOException Thronw if there is an error connecting to Bigquery.
     */
    public static Bigquery getService() throws IOException {
      if (service == null) {
        synchronized (serviceLock) {
          if (service == null) {
            service = createAuthorizedClient();
          }
        }
      }
      return service;
    }

    /**
     * Creates an authorized client to Google Bigquery.
     *
     * @return The BigQuery Service
     * @throws IOException Thrown if there is an error connecting
     */
    // [START get_service]
    private static Bigquery createAuthorizedClient() throws IOException {
      // Create the credential
      HttpTransport transport = new NetHttpTransport();
      JsonFactory jsonFactory = new JacksonFactory();
      GoogleCredential credential =  GoogleCredential.getApplicationDefault(transport, jsonFactory);

      // Depending on the environment that provides the default credentials (e.g. Compute Engine, App
      // Engine), the credentials may require us to specify the scopes we need explicitly.
      // Check for this case, and inject the Bigquery scope if required.
      if (credential.createScopedRequired()) {
        Collection<String> bigqueryScopes = BigqueryScopes.all();
        credential = credential.createScoped(bigqueryScopes);
      }

      return new Bigquery.Builder(transport, jsonFactory, credential)
          .setApplicationName("Data processing storm").build();
    }
    // [END get_service]

  }

  public static void submit_to_bigquery(JsonReader rows) throws IOException {
    final Gson gson = new Gson();
    Map<String, Object> rowData = gson.<Map<String, Object>>fromJson(rows, (new HashMap<String, Object>()).getClass());
    final TableDataInsertAllRequest.Rows row = new TableDataInsertAllRequest.Rows().setJson(rowData);
    bigquery.tabledata().insertAll(PROJECT_ID, DATASET_ID, TABLE_ID, new TableDataInsertAllRequest().setRows(Collections.singletonList(row))).execute();
  }

  public static class StoreToBigQueryBolt extends BaseRichBolt {
    OutputCollector _collector;

    @Override
    public void prepare(Map conf, TopologyContext context, OutputCollector collector) {
      _collector = collector;
      try {
        bigquery = BigqueryServiceFactory.getService();
      } catch (IOException e) {
        e.printStackTrace();
      }
    }

    @Override
    public void execute(Tuple tuple) {
      String json = tuple.getString(0);
      // Do processing
      // Send it to bigquery
      try {
        submit_to_bigquery(new JsonReader(new StringReader(json)));
      } catch (IOException e) {
        e.printStackTrace();
      }
      // Pass it on
      // _collector.emit(tuple, new Values(json));
      // Acknowledge it
      _collector.ack(tuple);
    }

    @Override
    public void declareOutputFields(OutputFieldsDeclarer declarer) {
      declarer.declare(new Fields("word"));
    }
  }

  public static class MessageScheme implements Scheme {
    @Override
    public List<Object> deserialize(byte[] ser) {
      List<Object> objects = new ArrayList<Object>();
      objects.add(new String(ser));
      return objects;
    }

    @Override
    public Fields getOutputFields() {
      Fields fields = new Fields("json_string");
      return fields;
    }
  }

  public static void main(String[] args) throws Exception {
    TopologyBuilder builder = new TopologyBuilder();
    Scheme scheme = new MessageScheme();
    IRichSpout spout = new RabbitMQSpout(scheme);
    ConnectionConfig connectionConfig = new ConnectionConfig(RABBIT_HOST, RABBIT_PORT, RABBIT_USERNAME, RABBIT_PASSWORD, ConnectionFactory.DEFAULT_VHOST, 580); // host, port, username, password, virtualHost, heartBeat
    ConsumerConfig spoutConfig = new ConsumerConfigBuilder().connection(connectionConfig)
                                                            .queue("messages")
                                                            .prefetch(200)
                                                            .requeueOnFail()
                                                            .build();
    builder.setSpout("rabbitmq", spout, 100)
           .setNumTasks(100)
           .addConfigurations(spoutConfig.asMap())
           .setMaxSpoutPending(200);
    builder.setBolt("process-message", new StoreToBigQueryBolt(), 100).shuffleGrouping("rabbitmq").setNumTasks(100);

    Config conf = new Config();

    if (args != null && args.length > 0) {
      conf.setDebug(false);
      conf.setNumWorkers(14);
      StormSubmitter.submitTopologyWithProgressBar(args[0], conf, builder.createTopology());
    } else {
      conf.setDebug(true);
      LocalCluster cluster = new LocalCluster();
      cluster.submitTopology("test", conf, builder.createTopology());
      Utils.sleep(10000);
      cluster.killTopology("test");
      cluster.shutdown();
    }
  }
}
