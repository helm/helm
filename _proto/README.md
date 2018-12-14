Protobuf3 type declarations for the Helm API
--------------------------------------------

Packages

 - `hapi.chart` Complete serialization of Heml charts
 - `hapi.release` Information about installed charts (Releases) such as metadata about when they were installed, their status, and how they were configured.
 - `hapi.services.rudder` Definition for the ReleaseModuleService used by Tiller to manipulate releases on a given node
 - `hapi.services.tiller` Definition of the ReleaseService provided by Tiller and used by Helm clients to manipulate releases cluster wide.
 - `hapi.version` Version meta-data used by tiller to express it's version
