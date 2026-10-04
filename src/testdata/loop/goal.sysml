package SymphonyGoal {
    part def Component;
    part def SensorContract;
    port def DataContract {
        attribute shape = "v2";
    }
    part analytics : Component {
        attribute owner = "data";
    }
    part billing : Component {
        attribute owner = "finance";
    }
    part catalog : Component {
        attribute owner = "platform";
        attribute config_replicas = "2";
    }
    part ingest : Component {
        attribute owner = "platform";
        port input : DataContract;
    }
    part platform : Component {
        attribute owner = "platform";
    }
    part serve : Component {
        attribute owner = "platform";
        port input : DataContract;
    }
    requirement keepShape {
        attribute about = "DataContract";
        attribute choice = "v2";
        attribute alternative = "v3";
        attribute rationale = "consumers cannot migrate";
        attribute status = "rejected";
    }
    part gitSensor : SensorContract {
        attribute observes = "git,manifest,zarf,cluster";
        attribute aim = "testdata/loop";
        attribute outside = "commit message bodies";
    }
}
